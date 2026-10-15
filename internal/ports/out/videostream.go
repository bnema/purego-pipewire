package out

// VideoStreamAPI is the set of C entry points needed by the video source: the
// generated VideoAPI group plus the stream calls shared with playback. CAPI
// satisfies it.
type VideoStreamAPI interface {
	VideoAPI
	StreamPlaybackAPI
}
