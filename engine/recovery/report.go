package recovery

import (
	"fmt"
	"time"
)

func writeReport(sink Sink, rep *Report) {
	w, err := sink.Create(unique(sink, "Recovery report.txt"))
	if err != nil {
		return
	}
	defer w.Close()
	fmt.Fprintf(w, "DataDoc recovery report\r\n%s\r\n\r\n", time.Now().Format("2 January 2006 15:04"))
	fmt.Fprintf(w, "Recovered %d files (%s), %d failed, in %s.\r\n\r\n", rep.Recovered, FormatBytes(rep.Bytes), rep.Failed, rep.Duration.Round(time.Second))
	for _, o := range rep.Outcomes {
		status := "OK"
		switch {
		case o.Err != nil:
			status = "FAILED: " + o.Err.Error()
		case o.Health < 100:
			status = fmt.Sprintf("recovered, %d%% intact", o.Health)
		}
		if o.Dest == "" {
			fmt.Fprintf(w, "%s (%s) %s\r\n", o.Source, FormatBytes(o.Size), status)
		} else {
			fmt.Fprintf(w, "%s\r\n    -> %s (%s) %s\r\n", o.Source, o.Dest, FormatBytes(o.Size), status)
		}
		if o.Note != "" {
			fmt.Fprintf(w, "    %s\r\n", o.Note)
		}
	}
}
