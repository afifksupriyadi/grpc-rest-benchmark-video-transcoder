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
	Resolution720p: "1280:720", // 720p scale
	Resolution480p: "854:480",  // 480p scale
	Resolution360p: "640:360",  // 360p scale
}
