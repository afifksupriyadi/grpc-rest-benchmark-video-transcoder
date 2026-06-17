// Package processor provides video processing utilities using FFmpeg.
package processor

import (
	"bytes"
	"fmt"

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

// Process transcodes the input video into all supported output resolutions.
// It runs each resolution transcode in parallel using goroutines.
// - input is the raw video bytes received from gateway
// - returns TranscodeResult containing output bytes per resolution
func (p *FFmpegProcessor) Process(input model.VideoData) (*model.TranscodeResult, error) {
	type result struct {
		resolution string
		data       []byte
		err        error
	}

	resultCh := make(chan result, len(constant.FFmpegResolutionScales))

	for resolution, scale := range constant.FFmpegResolutionScales {
		resolution := resolution
		scale := scale

		go func() {
			data, err := transcodeToResolution(input.Data, scale)
			resultCh <- result{resolution: resolution, data: data, err: err}
		}()
	}

	transcodeResult := &model.TranscodeResult{}
	for range constant.FFmpegResolutionScales {
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
