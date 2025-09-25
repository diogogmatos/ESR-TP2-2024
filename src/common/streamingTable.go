package common

import (
	"fmt"
)

type StreamingEntry struct {
	Conn   string
	Stream string
}

type StreamingTable []StreamingEntry

// initializes and returns a new StreamingTable
func NewStreamingTable() StreamingTable {
	return make([]StreamingEntry, 0)
}

// Add a new entry to the streaming table
func (table *StreamingTable) AddEntry(entry StreamingEntry) {
	*table = append(*table, entry)
}

// Find all streams associated with a specific connection ID
func (table *StreamingTable) AllStreamsByConn(connID string) ([]string, bool) {
	var streams []string
	for _, entry := range *table {
		if entry.Conn == connID {
			streams = append(streams, entry.Stream)
		}
	}
	return streams, len(streams) > 0
}

// Find all connections associated with a specific stream ID
func (table *StreamingTable) AllConnsByStream(streamID string) ([]string, bool) {
	var conns []string
	for _, entry := range *table {
		if entry.Stream == streamID {
			conns = append(conns, entry.Conn)
		}
	}
	return conns, len(conns) > 0
}

// Find the first stream associated with a specific connection ID
func (table *StreamingTable) StreamForConn(connID string) (string, bool) {
	for _, entry := range *table {
		if entry.Conn == connID {
			return entry.Stream, true
		}
	}
	return "", false // Return empty string and false if no entry found
}

// Display all entries in the streaming table as a formatted string
func (table *StreamingTable) Show() string {
	result := ""
	for _, entry := range *table {
		result += fmt.Sprintf("Conn: %s, Stream: %s\n", entry.Conn, entry.Stream)
	}
	return result
}

func (table *StreamingTable) FilterInPlace(f func(StreamingEntry) bool) {

	// Start with an index to track the position of the next valid entry
	writeIndex := 0

	// Iterate over the original slice of entries
	for i := 0; i < len(*table); i++ {
		entry := (*table)[i]
		if f(entry) {
			// If the entry matches the filter, keep it in the slice
			(*table)[writeIndex] = (*table)[i]
			writeIndex++
		}
	}

	// Trim the slice to only contain the valid entries
	*table = (*table)[:writeIndex]
}
