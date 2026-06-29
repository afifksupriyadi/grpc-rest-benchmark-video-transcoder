package constant

// FFmpeg processing parameters.
const (
	FFmpegVideoCodec = "libx264" // video codec used for transcoding
	FFmpegAudioCodec = "aac"     // audio codec used for transcoding
	FFmpegPreset     = "fast"    // encoding speed preset
	FFmpegFormat     = "mp4"     // output container format
)

// FFmpegResolutionScales maps resolution labels to ffmpeg scale filter values.
var FFmpegResolutionScales = map[string]string{
	Resolution720p: "1280:720",
	Resolution480p: "854:480",
	Resolution360p: "640:360",
}

// FFmpegResolutionHeights maps resolution labels to their pixel heights.
// Used to filter out resolutions that are not lower than the input video's height.
var FFmpegResolutionHeights = map[string]int{
	Resolution720p: 720,
	Resolution480p: 480,
	Resolution360p: 360,
}
