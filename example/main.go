package main

import (
	"github.com/LiamHaworth/windows-events"
	"log"
	"os"
	"os/signal"
)

var (
	exitChan   = make(chan os.Signal)
	errorsChan = make(chan error)
)

func main() {
	signal.Notify(exitChan, os.Interrupt)
	go interruptHandle()

	log.Println("Subscribing to windows logon events")
	eventSubscription := &windows_events.EventSubscription{
		Channel:         "Security",
		Query:           "*[EventData[Data[@Name='LogonType']='2'] and System[(EventID=4624)]]", // Successful interactive logon events
		SubscribeMethod: windows_events.EvtSubscribeToFutureEvents,
		Errors:          errorsChan,
		Callback:        eventCallback,
	}

	if err := eventSubscription.Create(); err != nil {
		log.Fatalf("Failed to create event subscription: %s", err)
		return
	}

	for err := range errorsChan {
		log.Printf("Event subscription error: %s", err)
	}

	if err := eventSubscription.Close(); err != nil {
		log.Fatalf("Encountered error while closing subscription: %s", err)
	} else {
		log.Println("Gracefully shutdown")
	}
}

func interruptHandle() {
	<-exitChan
	log.Println("Interrupt received from terminal, cleaning up and closing")
	close(exitChan)
	close(errorsChan)
}

func eventCallback(event *windows_events.Event) {
	targetDomain := event.FindEventData("TargetDomainName")
	targetUser := event.FindEventData("TargetUserName")

	if targetUser != nil && targetDomain != nil {
		log.Printf("User logon event received for [%s\\%s]", targetDomain.Value, targetUser.Value)
	}
}
