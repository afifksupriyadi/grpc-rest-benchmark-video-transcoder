package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/client/config"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/client/internal/constant"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/client/internal/model"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/client/internal/transport"
	grpctransport "github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/client/internal/transport/grpc"
	resttransport "github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/client/internal/transport/rest"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/shared/label"
)

var (
	protocolFlag         string
	inputFlag            string
	outputFlag           string
	scenarioFlag         string
	concurrencyLevelFlag string
)

// transcodeCmd sends a video file to the gateway for transcoding and saves the results locally.
var transcodeCmd = &cobra.Command{
	Use:   "transcode",
	Short: "Send a video to the gateway for transcoding",
	Long: `transcode reads a local video file, sends it to the Gateway service via
the chosen protocol, waits for the transcoded results, saves each resolution
as a separate file, then reports the final segment timing back to gateway.`,
	RunE: runTranscode,
}

func init() {
	transcodeCmd.Flags().StringVarP(&protocolFlag, "protocol", "p", "", "protocol to use: rest or grpc (required)")
	transcodeCmd.Flags().StringVarP(&inputFlag, "input", "i", "", "path to the input video file (required)")
	transcodeCmd.Flags().StringVarP(&outputFlag, "output", "o", constant.DefaultOutputDir, "directory to save transcoded results")
	transcodeCmd.Flags().StringVar(&scenarioFlag, "scenario", "", "research scenario identifier (e.g. scenario_a, scenario_b)")
	transcodeCmd.Flags().StringVar(&concurrencyLevelFlag, "concurrency-level", "", "concurrency level label, only relevant for scenario_b (e.g. 20)")

	if err := transcodeCmd.MarkFlagRequired("protocol"); err != nil {
		panic(err)
	}
	if err := transcodeCmd.MarkFlagRequired("input"); err != nil {
		panic(err)
	}

	rootCmd.AddCommand(transcodeCmd)
}

// runTranscode orchestrates the full client-side flow:
// read input -> record t1 -> send via chosen protocol -> record t8 immediately
// after receiving completes -> save files -> report t8-t7 back to gateway.
func runTranscode(cmd *cobra.Command, args []string) error {
	if protocolFlag != constant.ProtocolREST && protocolFlag != constant.ProtocolGRPC {
		return fmt.Errorf("invalid protocol %q: must be %q or %q", protocolFlag, constant.ProtocolREST, constant.ProtocolGRPC)
	}

	cfg := config.Get()

	data, err := os.ReadFile(inputFlag)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}
	filename := filepath.Base(inputFlag)

	labels := label.Labels{
		Scenario:         scenarioFlag,
		PayloadSize:      payloadSizeLabel(int64(len(data))),
		ConcurrencyLevel: concurrencyLevelFlag,
	}

	client, err := newGatewayClient(cfg, protocolFlag)
	if err != nil {
		return fmt.Errorf("failed to create gateway client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	// t1: client starts sending to gateway
	t1 := time.Now()

	result, t7, err := client.Transcode(ctx, filename, data, t1, labels)
	if err != nil {
		return fmt.Errorf("transcode request failed: %w", err)
	}

	// t8: client finishes receiving from gateway, recorded immediately,
	// BEFORE saving files to disk (saving is not part of the measured segment)
	t8 := time.Now()

	if err := saveResults(outputFlag, filename, result); err != nil {
		return fmt.Errorf("failed to save results: %w", err)
	}

	gatewayToClientDuration := t8.Sub(t7)
	totalBytes := totalResultSize(result)

	if err := reportMetric(cfg.GatewayRESTAddr, gatewayToClientDuration, totalBytes, protocolFlag, labels); err != nil {
		// reporting failure should not fail the whole command; results are already saved
		fmt.Fprintf(os.Stderr, "warning: failed to report metric to gateway: %v\n", err)
	}

	fmt.Printf("Transcoding complete. %d resolution(s) saved to %s\n", len(result.Outputs), outputFlag)
	return nil
}

// payloadSizeLabel rounds the input file size into a label like "10mb" or "500mb",
// matching the fixed payload sizes defined for Scenario A. Falls back to a raw
// byte-based label if the size doesn't match any predefined bucket.
func payloadSizeLabel(bytes int64) string {
	mb := bytes / (1024 * 1024)
	switch {
	case mb <= 10:
		return "10mb"
	case mb <= 50:
		return "50mb"
	case mb <= 100:
		return "100mb"
	case mb <= 250:
		return "250mb"
	case mb <= 500:
		return "500mb"
	default:
		return fmt.Sprintf("%dmb", mb)
	}
}

// newGatewayClient builds the appropriate GatewayClient implementation based on the chosen protocol.
func newGatewayClient(cfg *config.Config, protocol string) (transport.GatewayClient, error) {
	switch protocol {
	case constant.ProtocolREST:
		return resttransport.NewRestGatewayClient(cfg.GatewayRESTAddr, cfg.Timeout), nil
	case constant.ProtocolGRPC:
		return grpctransport.NewGrpcGatewayClient(cfg.GatewayGRPCAddr)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}
}

// saveResults writes each transcoded resolution output to its own file in outputDir.
// Filename pattern: <original-name-without-extension>_<resolution>.mp4
func saveResults(outputDir, originalFilename string, result *model.TranscodeResult) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	base := strings.TrimSuffix(originalFilename, filepath.Ext(originalFilename))

	for _, output := range result.Outputs {
		outPath := filepath.Join(outputDir, fmt.Sprintf("%s_%s.mp4", base, output.Resolution))
		if err := os.WriteFile(outPath, output.Data, 0o644); err != nil {
			return fmt.Errorf("failed to write %s: %w", outPath, err)
		}
	}

	return nil
}

// totalResultSize calculates the total bytes across all transcoded outputs.
func totalResultSize(result *model.TranscodeResult) int64 {
	var total int64
	for _, o := range result.Outputs {
		total += int64(len(o.Data))
	}
	return total
}

// reportMetricPayload mirrors gateway's ReportMetricRequest JSON shape.
// Defined locally because client cannot import gateway's internal package
// across module boundaries.
type reportMetricPayload struct {
	Segment          string  `json:"segment"`
	Protocol         string  `json:"protocol"`
	DurationSeconds  float64 `json:"durationSeconds"`
	Bytes            int64   `json:"bytes"`
	Scenario         string  `json:"scenario"`
	PayloadSize      string  `json:"payloadSize"`
	ConcurrencyLevel string  `json:"concurrencyLevel"`
}

// reportMetric sends the client-measured SegmentGatewayToClient duration back to
// gateway, since gateway has no way to measure t8 (client's own receive-complete
// time) on its own. Scenario labels are included so gateway can attach them to
// the correct histogram bucket.
func reportMetric(gatewayRESTAddr string, duration time.Duration, totalBytes int64, protocol string, labels label.Labels) error {
	payload := reportMetricPayload{
		Segment:          constant.SegmentGatewayToClient,
		Protocol:         protocol,
		DurationSeconds:  duration.Seconds(),
		Bytes:            totalBytes,
		Scenario:         labels.Scenario,
		PayloadSize:      labels.PayloadSize,
		ConcurrencyLevel: labels.ConcurrencyLevel,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal report payload: %w", err)
	}

	url := fmt.Sprintf("http://%s/v1/report-metric", gatewayRESTAddr)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to send report: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("gateway returned unexpected status: %d", resp.StatusCode)
	}

	return nil
}
