package homepage

import (
	"net/http"
	"strconv"
	"strings"

	"element-skin/backend/internal/util"
)

type UploadInput struct {
	Filename string
	Data     []byte
	Fields   map[string]string
}

func ParseMediaValues(fields map[string]string, typ string) (MediaValues, error) {
	values := MediaValues{
		StartYaw:      0,
		StartPitch:    0,
		YawSpeedDPS:   4,
		PitchSpeedDPS: 0,
		DurationMS:    intField(fields, "duration_ms", 0),
	}
	var err error
	values.OverlayOpacityLight, err = floatField(fields, "overlay_opacity_light", 0.45)
	if err != nil {
		return MediaValues{}, util.HTTPError{Status: http.StatusBadRequest, Object: "homepage_light_opacity", Operation: "validate", Reason: "invalid"}
	}
	values.OverlayOpacityDark, err = floatField(fields, "overlay_opacity_dark", 0.45)
	if err != nil {
		return MediaValues{}, util.HTTPError{Status: http.StatusBadRequest, Object: "homepage_dark_opacity", Operation: "validate", Reason: "invalid"}
	}
	if err := validateOpacityValue("overlay_opacity_light", values.OverlayOpacityLight); err != nil {
		return MediaValues{}, err
	}
	if err := validateOpacityValue("overlay_opacity_dark", values.OverlayOpacityDark); err != nil {
		return MediaValues{}, err
	}
	if typ != "panorama" {
		return values, nil
	}
	if values.StartYaw, err = floatField(fields, "start_yaw", 0); err != nil {
		return MediaValues{}, util.HTTPError{Status: http.StatusBadRequest, Object: "homepage_start_yaw", Operation: "validate", Reason: "invalid"}
	}
	if values.StartPitch, err = floatField(fields, "start_pitch", 0); err != nil {
		return MediaValues{}, util.HTTPError{Status: http.StatusBadRequest, Object: "homepage_start_pitch", Operation: "validate", Reason: "invalid"}
	}
	if values.YawSpeedDPS, err = floatField(fields, "yaw_speed_dps", 4); err != nil {
		return MediaValues{}, util.HTTPError{Status: http.StatusBadRequest, Object: "homepage_yaw_speed", Operation: "validate", Reason: "invalid"}
	}
	if values.PitchSpeedDPS, err = floatField(fields, "pitch_speed_dps", 0); err != nil {
		return MediaValues{}, util.HTTPError{Status: http.StatusBadRequest, Object: "homepage_pitch_speed", Operation: "validate", Reason: "invalid"}
	}
	if err := ValidatePanoramaValues(&values.StartYaw, &values.StartPitch, &values.YawSpeedDPS, &values.PitchSpeedDPS); err != nil {
		return MediaValues{}, err
	}
	return values, nil
}

func ValidateOpacity(name string, v *float64) error {
	if v == nil {
		return nil
	}
	return validateOpacityValue(name, *v)
}

func validateOpacityValue(name string, v float64) error {
	if v < 0 || v > 0.9 {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "homepage_setting", Operation: "validate", Reason: "out_of_range", Params: map[string]any{"field": name}}
	}
	return nil
}

func ValidatePanoramaValues(startYaw, startPitch, yawSpeedDPS, pitchSpeedDPS *float64) error {
	if startYaw != nil && (*startYaw < -360 || *startYaw > 360) {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "homepage_start_yaw", Operation: "validate", Reason: "out_of_range"}
	}
	if startPitch != nil && (*startPitch < -89 || *startPitch > 89) {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "homepage_start_pitch", Operation: "validate", Reason: "out_of_range"}
	}
	if yawSpeedDPS != nil && (*yawSpeedDPS < -90 || *yawSpeedDPS > 90) {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "homepage_yaw_speed", Operation: "validate", Reason: "out_of_range"}
	}
	if pitchSpeedDPS != nil && (*pitchSpeedDPS < -90 || *pitchSpeedDPS > 90) {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "homepage_pitch_speed", Operation: "validate", Reason: "out_of_range"}
	}
	return nil
}

func floatField(fields map[string]string, key string, fallback float64) (float64, error) {
	raw := strings.TrimSpace(fields[key])
	if raw == "" {
		return fallback, nil
	}
	return strconv.ParseFloat(raw, 64)
}

func intField(fields map[string]string, key string, fallback int) int {
	raw := strings.TrimSpace(fields[key])
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}
