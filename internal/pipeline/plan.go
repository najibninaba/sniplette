package pipeline

import "ig2wa/internal/model"

// PlanResolutionAndCRF computes the target long-side resolution (avoiding upscaling)
// and determines the CRF to use, given the chosen preset CRF.
func PlanResolutionAndCRF(opts model.CLIOptions, dv model.DownloadedVideo, presetCRF int) (int, int) {
	target := opts.Resolution
	if target <= 0 {
		// Fallback to 720 if unset
		target = 720
	}
	inLong := maxInt(dv.Width, dv.Height)
	if inLong > 0 && inLong < target {
		target = inLong
	}
	return target, presetCRF
}

// DefaultCRF maps a quality preset to a default CRF.
func DefaultCRF(q model.QualityPreset) int {
	switch q {
	case model.PresetLow:
		return 26
	case model.PresetHigh:
		return 19
	case model.PresetMedium:
		fallthrough
	default:
		return 22
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}