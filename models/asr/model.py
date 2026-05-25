import numpy as np
import triton_python_backend_utils as pb_utils
import whisper


class TritonPythonModel:
    def initialize(self, args):
        self.model = whisper.load_model("large-v3")

    def execute(self, requests):
        responses = []
        for request in requests:
            # audio_data: FP32 tensor of shape [1, N], values in [-1.0, 1.0]
            audio_tensor = pb_utils.get_input_tensor_by_name(request, "audio_data")
            audio = audio_tensor.as_numpy()[0].astype(np.float32)

            sample_rate_tensor = pb_utils.get_input_tensor_by_name(request, "sample_rate")
            sample_rate = int(sample_rate_tensor.as_numpy()[0])

            # Whisper expects float32 audio at 16 kHz — the worker already resampled.
            if sample_rate != whisper.audio.SAMPLE_RATE:
                raise ValueError(
                    f"expected sample rate {whisper.audio.SAMPLE_RATE}, got {sample_rate}"
                )

            result = self.model.transcribe(audio, fp16=True)
            transcript = result["text"].strip()

            out = pb_utils.Tensor(
                "transcript",
                np.array([[transcript.encode("utf-8")]], dtype=object),
            )
            responses.append(pb_utils.InferenceResponse(output_tensors=[out]))
        return responses

    def finalize(self):
        del self.model
