// Package processor provides video processing utilities using FFmpeg.
package processor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"

	ffmpeg "github.com/u2takey/ffmpeg-go"

	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/worker/internal/constant"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/worker/internal/model"
)

// FFmpegProcessor handles video transcoding using FFmpeg.
type FFmpegProcessor struct{}

// NewFFmpegProcessor creates a new FFmpegProcessor.
func NewFFmpegProcessor() *FFmpegProcessor {
	return &FFmpegProcessor{}
}

// Process transcodes the input video into all supported resolutions that are
// strictly lower than the input's own height. It probes the input first, then
// runs each qualifying resolution transcode in parallel using goroutines.
func (p *FFmpegProcessor) Process(input model.VideoData) (*model.TranscodeResult, error) {
	inputHeight, err := probeHeight(input.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to probe input video height: %w", err)
	}

	targetScales := make(map[string]string)
	for resolution, scale := range constant.FFmpegResolutionScales {
		if constant.FFmpegResolutionHeights[resolution] < inputHeight {
			targetScales[resolution] = scale
		}
	}

	if len(targetScales) == 0 {
		return nil, fmt.Errorf("input resolution %dpx has no supported lower output resolutions", inputHeight)
	}

	type result struct {
		resolution string
		data       []byte
		err        error
	}

	resultCh := make(chan result, len(targetScales))

	for resolution, scale := range targetScales {
		resolution := resolution
		scale := scale

		go func() {
			data, err := transcodeToResolution(input.Data, scale)
			resultCh <- result{resolution: resolution, data: data, err: err}
		}()
	}

	transcodeResult := &model.TranscodeResult{}
	for range targetScales {
		res := <-resultCh
		if res.err != nil {
			return nil, fmt.Errorf("transcoding failed for %s: %w", res.resolution, res.err)
		}
		transcodeResult.Outputs = append(transcodeResult.Outputs, model.ResolutionOutput{
			Resolution: res.resolution,
			Data:       res.data,
		})
	}

	return transcodeResult, nil
}

// probeHeight reads the height of the first video stream from raw video bytes
// by piping them into ffprobe.
func probeHeight(data []byte) (int, error) {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-select_streams", "v:0",
		"pipe:0",
	)
	cmd.Stdin = bytes.NewReader(data)

	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	var probe struct {
		Streams []struct {
			Height int `json:"height"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &probe); err != nil {
		return 0, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}
	if len(probe.Streams) == 0 {
		return 0, fmt.Errorf("no video streams found in input")
	}

	return probe.Streams[0].Height, nil
}

// transcodeToResolution transcodes raw video bytes to the given scale using FFmpeg.
// It reads input from memory and writes output to memory without touching the filesystem.
func transcodeToResolution(input []byte, scale string) ([]byte, error) {
	inputBuf := bytes.NewReader(input)
	outputBuf := &bytes.Buffer{}

	err := ffmpeg.
		Input("pipe:0").
		Output("pipe:1",
			ffmpeg.KwArgs{
				"vf":       fmt.Sprintf("scale=%s", scale),
				"vcodec":   constant.FFmpegVideoCodec,
				"acodec":   constant.FFmpegAudioCodec,
				"preset":   constant.FFmpegPreset,
				"format":   constant.FFmpegFormat,
				"movflags": "frag_keyframe+empty_moov",
			},
		).
		WithInput(inputBuf).
		WithOutput(outputBuf).
		Run()

	if err != nil {
		return nil, fmt.Errorf("ffmpeg error: %w", err)
	}

	return outputBuf.Bytes(), nil
}
