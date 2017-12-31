package skyaway

import (
	"fmt"
	"log"
	"time"
)

type task int

const (
	nothing task = iota
	announceEventStart
	startEvent
	announceEventEnd
	endEvent
)

// Returns what to do next (start, stop or nothing) and when
func (bot *Bot) schedule() (task, time.Time) {
	event := bot.db.GetCurrentEvent()
	if event == nil {
		return nothing, time.Time{}
	}

	if event.StartedAt.Valid {
		return endEvent, event.StartedAt.Time.Add(event.Duration.Duration)
	} else if event.ScheduledAt.Valid {
		return startEvent, event.ScheduledAt.Time
	}

	log.Print("The current event is not scheduled, not started and not ended. That should not have happened.")
	return nothing, time.Time{}

}

// Returns a more detailed version than `schedule()`
// of what to do next (including announcements).
func (bot *Bot) subSchedule() (task, time.Time) {
	tsk, future := bot.schedule()
	if tsk == nothing {
		return nothing, time.Time{}
	}

	every := bot.config.AnnounceEvery.Duration

	announcements := time.Until(future) / every
	if announcements <= 0 {
		return tsk, future
	}

	nearFuture := future.Add(-announcements * every)
	switch tsk {
	case startEvent:
		return announceEventStart, nearFuture
	case endEvent:
		return announceEventEnd, nearFuture
	default:
		log.Print("unsupported task to subSchedule")
		return nothing, time.Time{}
	}
}

func (bot *Bot) perform(tsk task) {
	event := bot.db.GetCurrentEvent()
	if event == nil {
		log.Print("failed to perform the scheduled task: no current event")
		return
	}

	noctx := &Context{}
	switch tsk {
	case announceEventStart:
		if event.Surprise {
			log.Printf("not announcing the future start of a surprise event")
			break
		}

		log.Print("announcing the event future start")
		if err := bot.AnnounceEventWithTitle(event, "Event is scheduled"); err != nil {
			log.Printf("failed to announce event future start: %v", err)
		}
	case announceEventEnd:
		log.Print("announcing the event future end")
		if err := bot.AnnounceEventWithTitle(event, "Event is ongoing"); err != nil {
			log.Printf("failed to announce event future end: %v", err)
		}
	case startEvent:
		log.Print("starting the event")

		startedEvent, err := bot.StartCurrentEvent()
		if err != nil {
			log.Printf("failed to start event: %v", err)
			break
		}

		md := formatEventAsMarkdown(startedEvent, true)
		md = fmt.Sprintf("*%s*\n%s", "Event has started!", md)
		if err := bot.Send(noctx, "yell", "markdown", md); err != nil {
			log.Printf("failed to announce event started: %v", err)
		}
	case endEvent:
		log.Print("ending the event")

		endedEvent, err := bot.EndCurrentEvent()
		if err != nil {
			log.Printf("failed to end event: %v", err)
			break
		}

		md := formatEventAsMarkdown(endedEvent, true)
		md = fmt.Sprintf("*%s*\n%s", "Event has ended!", md)
		if err := bot.Send(noctx, "yell", "markdown", md); err != nil {
			log.Printf("failed to announce event ended: %v", err)
		}
	default:
		log.Printf("unsupported task to perform: %v", tsk)
	}
}

func (bot *Bot) maintain() {
	bot.rescheduleChan = make(chan int)
	defer func() {
		close(bot.rescheduleChan)
	}()

	var timer *time.Timer
	for {
		tsk, future := bot.subSchedule()
		if tsk == nothing {
			<-bot.rescheduleChan
			continue
		}

		if timer == nil {
			timer = time.NewTimer(time.Until(future))
		} else {
			timer.Reset(time.Until(future))
		}
		select {
		case <-timer.C:
			bot.perform(tsk)
		case <-bot.rescheduleChan:
			if !timer.Stop() {
				<-timer.C
			}
		}
	}
}

// Cause a reschedule to happen. Call this if you modify events, so that the
// bot could wake itself up at correct times for automatic announcements and
// event starting/stopping.
func (bot *Bot) Reschedule() {
	bot.rescheduleChan <- 1
}
