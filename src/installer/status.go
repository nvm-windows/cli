package installer

import (
	"errors"
	"fmt"
	"nvm/log"
	"nvm/status"
	"strings"
	"time"

	"slices"
)

var emptyStatusInt int

type Status struct {
	spinner          *status.Spinner
	Versions         []string
	Downloads        int
	DownloadPct      int
	TotalDownloads   int
	Extractions      int
	TotalExtractions int
	ExtractionPct    int
	Verifying        int
	TotalCached      int
	Npm              int
	NpmLabel         string
	NpmPct           int
	TotalInstalled   int
	logs             []string
	start            time.Time
	running          bool
	cancelMessage    string
}

func newStatus() *Status {
	return &Status{
		Versions:         []string{},
		Downloads:        0,
		TotalDownloads:   0,
		Extractions:      0,
		TotalExtractions: 0,
		Verifying:        0,
		TotalCached:      0,
		TotalInstalled:   0,
		Npm:              0,
		NpmLabel:         "Installing modules",
		running:          false,
		spinner:          status.NewSpinner(""),
	}
}

func (s *Status) Flush() {
	s.update()
}

func (s *Status) update() {
	if s.cancelMessage != "" {
		s.spinner.Label = s.cancelMessage
		return
	}

	// if s.DownloadPct != emptyInt || s.NpmPct != emptyInt || s.ExtractionPct != emptyInt {
	// 	label := fmt.Sprintf("Download: %d%% | Extraction: %d%%", s.DownloadPct, s.ExtractionPct)

	// 	if s.NpmPct != emptyStatusInt {
	// 		label = fmt.Sprintf("%s | npm: %d%%", label, s.NpmPct)
	// 	}

	// 	s.spinner.Label = fmt.Sprintf("%s %v", label, time.Since(s.start).Truncate(time.Second))

	// 	return
	// }

	plural := ""
	if len(s.Versions) != 1 {
		plural = "s"
	}

	prefix := fmt.Sprintf("Installing %d version%s", len(s.Versions), plural)
	actions := []string{}

	if s.Downloads > 0 {
		actions = append(actions, "Downloading")
	}

	if s.Extractions > 0 {
		actions = append(actions, "Extracting")
	}

	if s.Verifying > 0 {
		actions = append(actions, "Verifying")
	}

	if s.Npm > 0 && s.NpmLabel != "" && len(strings.TrimSpace(s.NpmLabel)) > 0 {
		actions = append(actions, s.NpmLabel)
	}

	if len(actions) == 0 {
		actions = append(actions, "Waiting")
	}

	s.spinner.Label = fmt.Sprintf("%s: %s... %v", prefix, strings.Join(actions, " | "), time.Since(s.start).Truncate(time.Second))
}

func (s *Status) Start(interval ...time.Duration) {
	if s.running {
		return
	}

	i := 100 * time.Millisecond
	if len(interval) > 0 {
		i = interval[0]
	}

	s.spinner.Start()
	s.running = true
	s.start = time.Now()

	go func() {
		for {
			if !s.running {
				return
			}

			s.update()
			time.Sleep(i)
		}
	}()
}

func (s *Status) Log(message string) {
	s.logs = append(s.logs, message)
}

func (s *Status) Logf(format string, args ...interface{}) {
	s.Log(fmt.Sprintf(format, args...))
}

func (s *Status) Alert(value interface{}, logEvent ...bool) {
	_logEvent := true
	if len(logEvent) > 0 {
		_logEvent = logEvent[0]
	}

	msg := ""
	iserr := false

	switch v := value.(type) {
	case error:
		msg = v.Error()
		iserr = true
	case string:
		msg = v
	default:
		msg = fmt.Sprintf("%v", v)
	}

	s.spinner.PrintBefore(msg)
	if _logEvent {
		if iserr {
			log.Error(value.(error))
		} else {
			log.Warn(msg)
		}
	}
}

func (s *Status) Abort(value interface{}, displayError ...bool) error {
	var err error
	switch v := value.(type) {
	case error:
		err = v
	case string:
		err = errors.New(v)
	default:
		err = fmt.Errorf("%v", v)
	}

	if len(displayError) == 0 || displayError[0] {
		s.Log(err.Error())
	}

	s.Done()

	return err
}

func (s *Status) Cancel(message ...string) {
	s.cancelMessage = "Cancelled: rolling back..."
	if len(message) > 0 {
		s.cancelMessage = message[0]
	}
}

func (s *Status) Done(fn ...func(args ...string)) {
	if !s.running {
		return
	}

	s.spinner.Stop()
	s.running = false

	if s.cancelMessage != "" {
		return
	}

	if s.TotalCached > 0 {
		s.Logf("Cached %d version%s.", s.TotalCached, plural(s.TotalCached))
	}

	if s.TotalInstalled > 0 {
		versions := slices.DeleteFunc(s.Versions, func(v string) bool { return strings.TrimSpace(v) == "" })

		for _, version := range versions {
			log.Logf("Installed Node.js v%s", version)
		}

		s.Logf("Installed %d version%s: %s", s.TotalInstalled, plural(s.TotalInstalled), strings.Join(versions, ", "))
	}

	if len(fn) > 0 {
		fn[0](s.logs...)
	} else {
		for _, log := range s.logs {
			fmt.Println(log)
		}
	}

	elapsed := time.Since(s.start).Truncate(time.Second)
	if elapsed > 0 && (s.TotalInstalled > 0 || s.TotalCached > 0) {
		fmt.Printf("Completed in %v\n", elapsed)
	}
}

func plural(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}
