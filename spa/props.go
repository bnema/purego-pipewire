package spa

// Prop identifies a property of SPA_TYPE_OBJECT_Props, used with their
// Value helper to determine the values.
type Prop uint32

// FloatProp is a Prop whose SPA type is Float, Int, Double, or Long.
type FloatProp Prop

func (p FloatProp) Value(v float32) PropValue {
	return PropValue{ID: Prop(p), Values: []float32{v}}
}

// BoolProp is a Prop whose SPA type is Bool.
type BoolProp Prop

func (p BoolProp) Value(v bool) PropValue {
	var f float32
	if v {
		f = 1
	}
	return PropValue{ID: Prop(p), Values: []float32{f}}
}

// FloatArrayProp is a Prop whose SPA type is an Array of Float.
type FloatArrayProp Prop

func (p FloatArrayProp) Value(values ...float32) PropValue {
	return PropValue{ID: Prop(p), Values: values}
}

// PropValue pairs a Prop id with the float32 payload to be transported.
type PropValue struct {
	ID     Prop
	Values []float32
}

// ControlInfo describes a stream control's current value and bounds, as
// returned by pw_stream_get_control. It mirrors struct pw_stream_control
// from <pipewire/stream.h>.
type ControlInfo struct {
	Name   string
	Def    float32
	Min    float32
	Max    float32
	Values []float32
}

// Audio related properties who don't have an underlying SPA type
// must have their values constructed per context.
const (
	PropWaveType    Prop = 0x10001 // SPA_PROP_waveType: audio test signal waveform (Id)
	PropPatternType Prop = 0x10005 // SPA_PROP_patternType: audio/video test pattern type (Id)
	PropDitherType  Prop = 0x10006 // SPA_PROP_ditherType: dithering algorithm applied on output (Id)

	// PropChannelMap is a channel map (Array of Id enum spa_audio_channel).
	PropChannelMap Prop = 0x1000B // SPA_PROP_channelMap

	// PropIEC958Codecs is the set of enabled IEC958 (S/PDIF) codecs
	// (Array of Id enum spa_audio_iec958_codec).
	PropIEC958Codecs Prop = 0x10011 // SPA_PROP_iec958Codecs

	PropVolumeRampScale Prop = 0x10016 // SPA_PROP_volumeRampScale: scale/curve used to ramp the volume (Id)
)

const (
	PropFrequency FloatProp = 0x10002 // SPA_PROP_frequency: audio test signal frequency in Hz (Float)

	// PropVolume is a volume: 0.0 silence, 1.0 no attenuation.
	PropVolume FloatProp = 0x10003 // SPA_PROP_volume

	PropVolumeBase FloatProp = 0x10009 // SPA_PROP_volumeBase: volume base
	PropVolumeStep FloatProp = 0x1000A // SPA_PROP_volumeStep: volume step

	PropLatencyOffsetNsec FloatProp = 0x1000E // SPA_PROP_latencyOffsetNsec: delay adjustment in nanoseconds (Long)

	PropVolumeRampSamples     FloatProp = 0x10012 // SPA_PROP_volumeRampSamples: samples to ramp the volume over (Int)
	PropVolumeRampStepSamples FloatProp = 0x10013 // SPA_PROP_volumeRampStepSamples: incremental samples per volume ramp step (Int)
	PropVolumeRampTime        FloatProp = 0x10014 // SPA_PROP_volumeRampTime: time in milliseconds to ramp the volume over (Int)
	PropVolumeRampStepTime    FloatProp = 0x10015 // SPA_PROP_volumeRampStepTime: incremental time in nanoseconds per volume ramp step (Long)
)

const (
	// PropMute is a mute switch.
	PropMute BoolProp = 0x10004 // SPA_PROP_mute

	PropTruncate BoolProp = 0x10007 // SPA_PROP_truncate: truncate samples instead of dithering

	// PropMonitorMute is a mute switch for the monitor ports.
	PropMonitorMute BoolProp = 0x1000C // SPA_PROP_monitorMute

	// PropSoftMute is a mute switch applied in software.
	PropSoftMute BoolProp = 0x1000F // SPA_PROP_softMute
)

const (
	// PropChannelVolumes is a volume array, one (linear) volume per channel.
	// 0.0 is silence, 1.0 is without attenuation.
	PropChannelVolumes FloatArrayProp = 0x10008 // SPA_PROP_channelVolumes

	// PropMonitorVolumes is a volume array, one (linear) volume per channel,
	// for the monitor ports.
	PropMonitorVolumes FloatArrayProp = 0x1000D // SPA_PROP_monitorVolumes

	// PropSoftVolumes is a volume array, one (linear) volume per channel.
	// 0.0 is silence, 1.0 is without attenuation. This is the volume applied
	// in software; some of it might also be applied in hardware (see
	// PropChannelVolumes).
	PropSoftVolumes FloatArrayProp = 0x10010 // SPA_PROP_softVolumes
)
