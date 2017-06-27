package windows_events

import "encoding/xml"

// Event represents an Windows Event
// parsed from XML
type Event struct {
	XMLName   xml.Name     `xml:"Event"`
	EventData []*EventData `xml:"EventData>Data"`
}

// FindEventData will lookup the eventData
// slice to find the first eventData entry
// that has a matching key
func (event *Event) FindEventData(key string) *EventData {
	for _, ed := range event.EventData {
		if ed.Key == key {
			return ed
		}
	}

	return nil
}

// EventData represents a `Data` element
// found in `EventData` of an Event XML
type EventData struct {
	Key   string `xml:"Name,attr"`
	Value string `xml:",chardata"`
}
