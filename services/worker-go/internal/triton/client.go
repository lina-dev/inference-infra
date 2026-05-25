package triton

import (
	"container/heap"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/inference-infra/worker/internal/chunker"
	pb "github.com/triton-inference-server/client/src/grpc_generated_v2/grpc-service-proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client sends audio chunks to Triton over gRPC.
type Client struct {
	conn        *grpc.ClientConn
	stub        pb.GRPCInferenceServiceClient
	concurrency int
}

func NewClient(addr string, concurrency int) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial triton %s: %w", addr, err)
	}
	if concurrency <= 0 {
		concurrency = 4
	}
	return &Client{
		conn:        conn,
		stub:        pb.NewGRPCInferenceServiceClient(conn),
		concurrency: concurrency,
	}, nil
}

func (c *Client) Close() error { return c.conn.Close() }

// Segment is a single timestamped transcript fragment from one chunk.
type Segment struct {
	ChunkIndex int
	StartSec   float64
	EndSec     float64
	Text       string
}

// Transcribe dispatches chunks concurrently (bounded by concurrency) and streams
// segments in ChunkIndex order via the returned channel.
func (c *Client) Transcribe(ctx context.Context, chunks []chunker.Chunk) (<-chan Segment, <-chan error) {
	outCh := make(chan Segment, c.concurrency)
	errCh := make(chan error, 1)

	go func() {
		defer close(outCh)
		if err := c.run(ctx, chunks, outCh); err != nil {
			errCh <- err
		}
	}()

	return outCh, errCh
}

func (c *Client) run(ctx context.Context, chunks []chunker.Chunk, outCh chan<- Segment) error {
	rawCh := make(chan Segment, c.concurrency)
	workerErrs := make(chan error, len(chunks))

	var wg sync.WaitGroup
	sem := make(chan struct{}, c.concurrency)

	go func() {
		for _, chunk := range chunks {
			select {
			case <-ctx.Done():
				return
			case sem <- struct{}{}:
			}
			wg.Add(1)
			go func(chunk chunker.Chunk) {
				defer wg.Done()
				defer func() { <-sem }()
				text, err := c.inferChunk(ctx, chunk)
				if err != nil {
					workerErrs <- fmt.Errorf("chunk %d: %w", chunk.Index, err)
					return
				}
				rawCh <- Segment{
					ChunkIndex: chunk.Index,
					StartSec:   chunk.StartSec,
					EndSec:     chunk.EndSec,
					Text:       text,
				}
			}(chunk)
		}
		wg.Wait()
		close(rawCh)
	}()

	buf := &reorderBuffer{out: outCh}
	for seg := range rawCh {
		buf.push(seg)
	}

	select {
	case err := <-workerErrs:
		return err
	default:
		return nil
	}
}

func (c *Client) inferChunk(ctx context.Context, chunk chunker.Chunk) (string, error) {
	req := &pb.ModelInferRequest{
		ModelName: "whisper",
		Id:        fmt.Sprintf("chunk-%d", chunk.Index),
		Inputs: []*pb.ModelInferRequest_InferInputTensor{
			{
				Name:     "audio_data",
				Datatype: "FP32",
				Shape:    []int64{1, int64(len(chunk.Data))},
				Contents: &pb.InferTensorContents{Fp32Contents: chunk.Data},
			},
			{
				Name:     "sample_rate",
				Datatype: "INT32",
				Shape:    []int64{1},
				Contents: &pb.InferTensorContents{IntContents: []int32{int32(chunk.SampleRate)}},
			},
		},
		Outputs: []*pb.ModelInferRequest_InferRequestedOutputTensor{
			{Name: "transcript"},
		},
	}

	stream, err := c.stub.ModelStreamInfer(ctx, req)
	if err != nil {
		return "", fmt.Errorf("stream infer: %w", err)
	}

	var text string
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("recv: %w", err)
		}
		if resp.ErrorMessage != "" {
			return "", fmt.Errorf("triton: %s", resp.ErrorMessage)
		}
		for _, out := range resp.InferResponse.Outputs {
			if out.Name == "transcript" && out.Contents != nil && len(out.Contents.BytesContents) > 0 {
				text += string(out.Contents.BytesContents[0])
			}
		}
	}
	return text, nil
}

// ---- reorder buffer (min-heap on ChunkIndex) --------------------------------

type reorderBuffer struct {
	pending      segmentHeap
	nextExpected int
	out          chan<- Segment
}

func (b *reorderBuffer) push(seg Segment) {
	heap.Push(&b.pending, seg)
	for len(b.pending) > 0 && b.pending[0].ChunkIndex == b.nextExpected {
		b.out <- heap.Pop(&b.pending).(Segment)
		b.nextExpected++
	}
}

type segmentHeap []Segment

func (h segmentHeap) Len() int           { return len(h) }
func (h segmentHeap) Less(i, j int) bool { return h[i].ChunkIndex < h[j].ChunkIndex }
func (h segmentHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *segmentHeap) Push(x any)        { *h = append(*h, x.(Segment)) }
func (h *segmentHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}
