package common

import (
	"encoding/json"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/olekukonko/tablewriter"
	"ser.uminho/tp2/helpers"
)

type RoutingEntry struct {
	From           net.Addr `json:"from"`
	To             net.Addr `json:"to"`
	ConnId         string   `json:"conn_id"`
	InUse          bool     `json:"in_use"`
	TotalHops      int      `json:"total_hops"`
	HopsSinceSplit int      `json:"hops_since_split"`
	LastHopTime    int      `json:"last_hop_time"`
	TimeFromSource int      `json:"time_from_source"`
}

func (re *RoutingEntry) String() string {
	if b, err := json.Marshal(re); err == nil {
		return string(b)
	} else {
		return "error converting to string"
	}

}

type RoutingTable struct {
	Entries []RoutingEntry
	MU      sync.RWMutex
}

func GetConnContent(connId string) string {
	var filename string
	// Use Sscanf with the appropriate format to parse the string
	connId = strings.Split(connId, "-")[0]
	if connId[0] == '[' {
		filename = connId[1 : len(connId)-1]
	} else {
		filename = connId
	}
	return filename
}

func GetConnParent(connId string) string {
	l := strings.Split(connId, "-")
	return strings.Join(l[0:len(l)-1], "-")
}

// NewRoutingTable initializes and returns a new RoutingTable
func NewRoutingTable() *RoutingTable {
	return &RoutingTable{
		Entries: make([]RoutingEntry, 0),
	}
}

// AddEntry adds a new entry to the routing table
func (table *RoutingTable) AddEntry(entry RoutingEntry) {
	table.MU.Lock()
	defer table.MU.Unlock()

	// Check if the entry already exists
	for n, existingEntry := range table.Entries {
		if (existingEntry.From == entry.From || helpers.CompareAddrs(existingEntry.From, entry.From)) &&
			(existingEntry.To == entry.To || helpers.CompareAddrs(existingEntry.To, entry.To)) &&
			existingEntry.ConnId == entry.ConnId {

			table.Entries[n] = entry
			// Entry already exists, do not add it
			return
		}
	}
	// If not found, add the new entry
	table.Entries = append(table.Entries, entry)
}

func (table *RoutingTable) AddEntryUnsafe(entry RoutingEntry) {
	// Check if the entry already exists
	for _, existingEntry := range table.Entries {
		if (existingEntry.From == entry.From || helpers.CompareAddrs(existingEntry.From, entry.From)) &&
			(existingEntry.To == entry.To || helpers.CompareAddrs(existingEntry.To, entry.To)) &&
			existingEntry.ConnId == entry.ConnId {
			// Entry already exists, do not add it
			return
		}
	}
	// If not found, add the new entry
	table.Entries = append(table.Entries, entry)
}

func (table *RoutingTable) Map(f func(RoutingEntry) RoutingEntry) {
	table.MU.Lock()
	defer table.MU.Unlock()

	for i, entry := range table.Entries {
		table.Entries[i] = f(entry)
	}
}

func (table *RoutingTable) FilterInPlace(f func(RoutingEntry) bool) []RoutingEntry {
	table.MU.Lock()
	defer table.MU.Unlock()

	// Start with an index to track the position of the next valid entry
	writeIndex := 0

	res := make([]RoutingEntry, 0)

	// Iterate over the original slice of entries
	for i := 0; i < len(table.Entries); i++ {
		entry := table.Entries[i]
		if f(entry) {
			// If the entry matches the filter, keep it in the slice
			table.Entries[writeIndex] = table.Entries[i]
			writeIndex++
		} else {
			res = append(res, entry)
		}
	}

	// Trim the slice to only contain the valid entries
	table.Entries = table.Entries[:writeIndex]
	return res
}

func (table *RoutingTable) PruneOrphans() []RoutingEntry {
	table.MU.Lock()
	defer table.MU.Unlock()

	sources := make(map[string]any)
	for _, re := range table.Entries {
		if re.From != nil {
			sources[re.ConnId] = 1
		}
	}
	res := make([]RoutingEntry, 0)
	writeIndex := 0

	for i := 0; i < len(table.Entries); i++ {
		entry := table.Entries[i]
		if _, ok := sources[GetConnParent(entry.ConnId)]; ok || entry.To == nil {
			table.Entries[writeIndex] = table.Entries[i]
			writeIndex++
		} else {

			res = append(res, entry)
		}
	}

	table.Entries = table.Entries[:writeIndex]
	return res
}

func (table *RoutingTable) Filter(f func(RoutingEntry) bool) []RoutingEntry {
	table.MU.RLock()
	defer table.MU.RUnlock()
	// Start with an index to track the position of the next valid entry
	writeIndex := 0
	res := make([]RoutingEntry, 0)
	// Iterate over the original slice of entries
	for i := 0; i < len(table.Entries); i++ {
		entry := table.Entries[i]
		if f(entry) {
			// If the entry matches the filter, keep it in the slice
			res = append(res, table.Entries[i])
			writeIndex++
		}
	}
	return res
}

func (table *RoutingTable) Count(f func(RoutingEntry) bool) int {
	table.MU.RLock()
	defer table.MU.RUnlock()
	res := 0
	for i := 0; i < len(table.Entries); i++ {
		entry := table.Entries[i]
		if f(entry) {
			res++
		}
	}
	return res
}

func (table *RoutingTable) Any(f func(RoutingEntry) bool) bool {
	table.MU.RLock()
	defer table.MU.RUnlock()

	for i := 0; i < len(table.Entries); i++ {
		entry := table.Entries[i]
		if f(entry) {
			return true
		}
	}
	return false
}

// AllFromIP finds all entries with the specified source IP (net.IPAddr type)
func (table *RoutingTable) AllFromIP(ip net.IPAddr) []RoutingEntry {
	table.MU.RLock()
	defer table.MU.RUnlock()

	var result []RoutingEntry
	helpers.WarnLogger.Printf("table has %d entries", len(table.Entries))
	for _, entry := range table.Entries {
		if addr, ok := entry.From.(*net.IPAddr); ok {
			helpers.WarnLogger.Printf("comparing %+v and %+v", addr.IP, ip.IP)
			if addr.IP.Equal(ip.IP) {
				helpers.WarnLogger.Printf("TRUE")

				result = append(result, entry)
			}
		}
	}
	return result
}

// Find all entries with the specified destination IP (net.IPAddr type)
func (table *RoutingTable) AllToIP(ip net.IPAddr) []RoutingEntry {
	table.MU.RLock()
	defer table.MU.RUnlock()
	result := make([]RoutingEntry, 0)
	for _, entry := range table.Entries {
		if addr, ok := entry.To.(*net.IPAddr); ok && addr.IP.Equal(ip.IP) {
			result = append(result, entry)
		}
	}
	return result
}

func (table *RoutingTable) GetContentSources(content string) []net.Addr {
	table.MU.RLock()
	defer table.MU.RUnlock()

	result := make([]net.Addr, 0)
	for _, entry := range table.Entries {
		if GetConnContent(entry.ConnId) == content && entry.From != nil {
			result = append(result, entry.From)
		}
	}
	return result
}

// Find the destination address for a given connection ID
func (table *RoutingTable) ForConn(connId string) net.Addr {
	table.MU.RLock()
	defer table.MU.RUnlock()
	for _, entry := range table.Entries {
		if entry.ConnId == connId {
			return entry.To
		}
	}
	return nil // Return nil if no entry found
}

func (table *RoutingTable) GetOutgoingForConn(connId string) []RoutingEntry {
	table.MU.RLock()
	defer table.MU.RUnlock()
	result := make([]RoutingEntry, 0)
	for _, entry := range table.Entries {
		if strings.HasPrefix(entry.ConnId, connId+"-") {
			if entry.To != nil {
				result = append(result, entry)
			}
		}
	}
	return result
}

func (table *RoutingTable) GetOutgoingForContent(content string) []RoutingEntry {
	table.MU.RLock()
	defer table.MU.RUnlock()
	result := make([]RoutingEntry, 0)
	for _, entry := range table.Entries {
		if GetConnContent(entry.ConnId) == (content) && entry.To != nil {
			if entry.To != nil {
				result = append(result, entry)
			}
		}
	}
	return result
}

func (table *RoutingTable) GetIncomingForContent(content string) []RoutingEntry {
	table.MU.RLock()
	defer table.MU.RUnlock()
	result := make([]RoutingEntry, 0)
	for _, entry := range table.Entries {
		if GetConnContent(entry.ConnId) == (content) && entry.From != nil {
			result = append(result, entry)

		}
	}
	return result
}

func (table *RoutingTable) AddOutgoingForConn(connId string, address net.Addr) string {
	table.MU.Lock()
	defer table.MU.Unlock()
	m := 0
	for _, entry := range table.Entries {
		if GetConnParent(entry.ConnId) == connId && helpers.CompareIps(entry.To, address) {
			return entry.ConnId
		}
		if strings.HasPrefix(entry.ConnId, connId+"-") {
			toks := strings.Split(entry.ConnId, "-")
			n, _ := strconv.Atoi(toks[len(toks)-1])
			m = max(m, n)
		}
	}
	suffix := m + 1
	table.AddEntryUnsafe(RoutingEntry{ConnId: connId + "-" + strconv.Itoa(suffix), To: address})
	return connId + "-" + strconv.Itoa(suffix)
}

func (table *RoutingTable) AddActiveOutgoingForConn(connId string, address net.Addr) string {
	table.MU.Lock()
	defer table.MU.Unlock()
	m := 0
	for _, entry := range table.Entries {
		if strings.HasPrefix(entry.ConnId, connId+"-") {
			toks := strings.Split(entry.ConnId, "-")
			n, _ := strconv.Atoi(toks[len(toks)-1])
			m = max(m, n)
		}
	}
	suffix := m + 1
	table.AddEntryUnsafe(RoutingEntry{ConnId: connId + "-" + strconv.Itoa(suffix), To: address, InUse: true})
	return connId + "-" + strconv.Itoa(suffix)
}

// check if a connection passed through "me". Used to avoid loops
func (table *RoutingTable) PassesThrough(connId string) bool {
	table.MU.RLock()
	defer table.MU.RUnlock()
	for _, entry := range table.Entries {
		if entry.To != nil && strings.HasPrefix(connId, entry.ConnId) {
			return true
		}
	}
	return false
}

// Show displays all entries in the routing table as a formatted string
func (table *RoutingTable) Show() {
	table.MU.RLock()
	defer table.MU.RUnlock()

	// Initialize tablewriter to write to the buffer
	tableWriter := tablewriter.NewWriter(os.Stdout)
	tableWriter.SetHeader([]string{"From", "To", "ConnId", "Hops", "Split at", "In Use", "LHTime", "STime"})
	tableWriter.SetBorders(tablewriter.Border{Left: true, Top: true, Right: true, Bottom: true})
	tableWriter.SetCenterSeparator("|")

	// Add rows to tablewriter
	for _, entry := range table.Entries {
		inUse := "No"
		if entry.InUse {
			inUse = "Yes"
		}
		var from string
		if entry.From == nil {
			from = "---"
		} else {
			from = strings.Split(entry.From.String(), ":")[0]
		}
		var to string
		if entry.To == nil {
			to = "---"
		} else {
			to = strings.Split(entry.To.String(), ":")[0]
		}

		row := []string{from, to, entry.ConnId, strconv.Itoa(entry.TotalHops), strconv.Itoa(entry.HopsSinceSplit), inUse, strconv.Itoa(entry.LastHopTime) + " ms", strconv.Itoa(entry.TimeFromSource) + "ms"}
		tableWriter.Append(row)
	}

	// Render the table to the output
	tableWriter.Render()
}

func ShowRoutingEntryArray(table []RoutingEntry) {
	// Initialize tablewriter to write to the buffer
	tableWriter := tablewriter.NewWriter(os.Stdout)
	tableWriter.SetHeader([]string{"From", "To", "ConnId", "Hops", "Split at", "In Use", "LHTime", "STime"})
	tableWriter.SetBorders(tablewriter.Border{Left: true, Top: true, Right: true, Bottom: true})
	tableWriter.SetCenterSeparator("|")

	// Add rows to tablewriter
	for _, entry := range table {
		inUse := "No"
		if entry.InUse {
			inUse = "Yes"
		}
		var from string
		if entry.From == nil {
			from = "---"
		} else {
			from = strings.Split(entry.From.String(), ":")[0]
		}
		var to string
		if entry.To == nil {
			to = "---"
		} else {
			to = strings.Split(entry.To.String(), ":")[0]
		}

		row := []string{from, to, entry.ConnId, strconv.Itoa(entry.TotalHops), strconv.Itoa(entry.HopsSinceSplit), inUse, strconv.Itoa(entry.LastHopTime) + " ms", strconv.Itoa(entry.TimeFromSource) + "ms"}
		tableWriter.Append(row)
	}

	// Render the table to the output
	tableWriter.Render()
}
func (table *RoutingTable) ShowUnsafe() {

	// Initialize tablewriter to write to the buffer
	tableWriter := tablewriter.NewWriter(os.Stdout)
	tableWriter.SetHeader([]string{"From", "To", "ConnId", "Hops", "Split at", "In Use", "LHTime", "STime"})
	tableWriter.SetBorders(tablewriter.Border{Left: true, Top: true, Right: true, Bottom: true})
	tableWriter.SetCenterSeparator("|")

	// Add rows to tablewriter
	for _, entry := range table.Entries {
		inUse := "No"
		if entry.InUse {
			inUse = "Yes"
		}
		var from string
		if entry.From == nil {
			from = "---"
		} else {
			from = strings.Split(entry.From.String(), ":")[0]
		}
		var to string
		if entry.To == nil {
			to = "---"
		} else {
			to = strings.Split(entry.To.String(), ":")[0]
		}

		row := []string{from, to, entry.ConnId, strconv.Itoa(entry.TotalHops), strconv.Itoa(entry.HopsSinceSplit), inUse, strconv.Itoa(entry.LastHopTime) + " ms", strconv.Itoa(entry.TimeFromSource) + "ms"}
		tableWriter.Append(row)
	}

	// Render the table to the output
	tableWriter.Render()
}

// ToTable returns a copy of the routing table as a slice of RoutingEntry
func (table *RoutingTable) ToTable() []RoutingEntry {
	table.MU.RLock()
	defer table.MU.RUnlock()

	// Create a copy of the entries
	copiedEntries := make([]RoutingEntry, len(table.Entries))
	copy(copiedEntries, table.Entries)

	return copiedEntries
}

func DegreeOfRemoval(RoutingTable *RoutingTable) int {
	min := 100000
	RoutingTable.Map(func(re RoutingEntry) RoutingEntry {
		if min > re.TotalHops {
			min = re.TotalHops
		}
		return re
	})
	return min
}
