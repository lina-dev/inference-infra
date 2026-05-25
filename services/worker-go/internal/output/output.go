package output

// Result is the final JSON written to S3.
type Result struct {
	JobID      string `json:"job_id"`
	Transcript string `json:"transcript"`
	Summary    string `json:"summary"`
}
