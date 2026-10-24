package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"player-stats-backend-go/internal/config"
	"player-stats-backend-go/internal/settings"
)

func decodeSettings(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32768))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		writeError(w, 400, "请求体不是有效配置")
		return false
	}
	if err := d.Decode(new(any)); err != io.EOF {
		writeError(w, 400, "请求体只能包含一个配置对象")
		return false
	}
	return true
}
func (s *Server) settingsSection(w http.ResponseWriter, r *http.Request) {
	var result any
	var err error
	switch r.PathValue("section") {
	case "smb":
		if r.Method == http.MethodGet {
			result, err = s.settingsService.SMB(r.Context())
		} else {
			var p settings.SMBSettings
			if !decodeSettings(w, r, &p) {
				return
			}
			result, err = s.settingsService.SaveSMB(r.Context(), p)
		}
	case "schedules":
		if r.Method == http.MethodGet {
			result, err = s.settingsService.Schedules(r.Context())
		} else {
			var p settings.ScheduleSettings
			if !decodeSettings(w, r, &p) {
				return
			}
			result, err = s.settingsService.SaveSchedules(r.Context(), p)
		}
	case "sharing":
		if r.Method == http.MethodGet {
			result, err = s.settingsService.Sharing(r.Context())
		} else {
			var p settings.SharingSettings
			if !decodeSettings(w, r, &p) {
				return
			}
			result, err = s.settingsService.SaveSharing(r.Context(), p)
		}
	case "runtime":
		if r.Method == http.MethodGet {
			result, err = s.settingsService.Runtime(r.Context())
		} else {
			var p config.RuntimeParameters
			if !decodeSettings(w, r, &p) {
				return
			}
			result, err = s.settingsService.SaveRuntime(r.Context(), p)
		}
	default:
		writeError(w, 404, "找不到设置分组")
		return
	}
	s.writeServiceResult(w, result, err)
}
