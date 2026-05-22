package biz

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMemorySamplerSampleCapturesHeapReleaseEvidence(t *testing.T) {
	samper := NewMemorySampler()
	samper.sample()

	snapshot := samper.Snapshot()
	require.Len(t, snapshot.History, 1)

	sample := snapshot.History[0]
	require.NotZero(t, sample.Timestamp)
	require.GreaterOrEqual(t, sample.HeapSys, sample.HeapInuse)
	require.GreaterOrEqual(t, sample.TotalAlloc, sample.HeapAlloc)
	require.GreaterOrEqual(t, sample.Mallocs, sample.Frees)
}

func TestMemorySamplerExportBundleCreatesZipWithProfilesAndSamples(t *testing.T) {
	samper := NewMemorySampler()
	samper.sample()

	bundle, err := samper.ExportBundle()
	require.NoError(t, err)
	require.NotEmpty(t, bundle)

	zr, err := zip.NewReader(bytes.NewReader(bundle), int64(len(bundle)))
	require.NoError(t, err)

	entries := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		entries[f.Name] = f
	}

	require.Contains(t, entries, "summary.json")
	require.Contains(t, entries, "current.json")
	require.Contains(t, entries, "samples.jsonl")
	require.Contains(t, entries, "heap.pprof")
	require.Contains(t, entries, "goroutines.txt")

	summary := readZipEntry(t, entries["summary.json"])
	var decoded struct {
		FormatVersion         int    `json:"formatVersion"`
		RetentionHours        int    `json:"retentionHours"`
		SampleIntervalSeconds int    `json:"sampleIntervalSeconds"`
		SampleCount           int    `json:"sampleCount"`
		ExportedAt            string `json:"exportedAt"`
	}
	require.NoError(t, json.Unmarshal(summary, &decoded))
	require.Equal(t, 1, decoded.FormatVersion)
	require.Equal(t, 24, decoded.RetentionHours)
	require.Equal(t, 600, decoded.SampleIntervalSeconds)
	require.Equal(t, 1, decoded.SampleCount)
	require.NotEmpty(t, decoded.ExportedAt)

	require.NotEmpty(t, readZipEntry(t, entries["current.json"]))
	require.NotEmpty(t, readZipEntry(t, entries["samples.jsonl"]))
	require.NotEmpty(t, readZipEntry(t, entries["heap.pprof"]))
	require.Contains(t, string(readZipEntry(t, entries["goroutines.txt"])), "goroutine")
}

func TestMemorySamplerStopIsIdempotent(t *testing.T) {
	sampler := NewMemorySampler()

	require.NotPanics(t, func() {
		sampler.Stop()
		sampler.Stop()
	})
}

func readZipEntry(t *testing.T, file *zip.File) []byte {
	t.Helper()

	r, err := file.Open()
	require.NoError(t, err)
	defer r.Close()

	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)
	return buf.Bytes()
}
