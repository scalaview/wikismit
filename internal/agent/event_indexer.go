package agent

import (
	"sort"
	"time"

	"github.com/scalaview/wikismit/pkg/store"
)

const defaultEventFactIndexVersion = "epic14/v1"

type EventIndexer struct {
	version string
	now     func() time.Time
}

func NewEventIndexer(version string, now func() time.Time) *EventIndexer {
	if version == "" {
		version = defaultEventFactIndexVersion
	}
	if now == nil {
		now = time.Now
	}
	return &EventIndexer{version: version, now: now}
}

func (e *EventIndexer) Build(idx store.FileIndex) store.EventFactIndex {
	if e == nil {
		e = NewEventIndexer("", nil)
	}

	aggregate := make(map[string]*store.EventEntry)
	paths := make([]string, 0, len(idx))
	for path := range idx {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		entry := idx[path]
		if entry == nil {
			continue
		}
		functions := append([]*store.FunctionDecl(nil), entry.Functions...)
		sort.Slice(functions, func(i int, j int) bool {
			return compareFunctionDecl(functions[i], functions[j]) < 0
		})
		for _, fn := range functions {
			if fn == nil || fn.EventFacts == nil {
				continue
			}
			e.addFacts(aggregate, fn.EventFacts.Publishes, eventRolePublishers)
			e.addFacts(aggregate, fn.EventFacts.Handles, eventRoleHandlers)
			e.addFacts(aggregate, fn.EventFacts.Registers, eventRoleRegistrations)
		}
	}

	events := make([]*store.EventEntry, 0, len(aggregate))
	for _, name := range sortedEventNames(aggregate) {
		event := aggregate[name]
		sortEventFacts(event.Publishers)
		sortEventFacts(event.Handlers)
		sortEventFacts(event.Registrations)
		events = append(events, event)
	}

	return store.EventFactIndex{
		Version:     e.version,
		GeneratedAt: e.now().UTC(),
		Events:      events,
	}
}

type eventRole int

const (
	eventRolePublishers eventRole = iota
	eventRoleHandlers
	eventRoleRegistrations
)

func (e *EventIndexer) addFacts(aggregate map[string]*store.EventEntry, facts []*store.EventFact, role eventRole) {
	for _, fact := range facts {
		if fact == nil || fact.EventName == "" {
			continue
		}
		entry := aggregate[fact.EventName]
		if entry == nil {
			entry = &store.EventEntry{EventName: fact.EventName}
			aggregate[fact.EventName] = entry
		}
		cloned := cloneEventFact(fact)
		switch role {
		case eventRolePublishers:
			entry.Publishers = append(entry.Publishers, cloned)
		case eventRoleHandlers:
			entry.Handlers = append(entry.Handlers, cloned)
		case eventRoleRegistrations:
			entry.Registrations = append(entry.Registrations, cloned)
		}
	}
}

func cloneEventFact(fact *store.EventFact) *store.EventFact {
	if fact == nil {
		return nil
	}
	return &store.EventFact{
		EventName:  fact.EventName,
		HandlerRef: fact.HandlerRef,
		FuncID:     fact.FuncID,
		Line:       fact.Line,
		Evidence:   fact.Evidence,
	}
}

func sortedEventNames(aggregate map[string]*store.EventEntry) []string {
	names := make([]string, 0, len(aggregate))
	for name := range aggregate {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortEventFacts(facts []*store.EventFact) {
	sort.Slice(facts, func(i int, j int) bool {
		return compareEventFacts(facts[i], facts[j]) < 0
	})
}

func compareFunctionDecl(left *store.FunctionDecl, right *store.FunctionDecl) int {
	if left == nil && right == nil {
		return 0
	}
	if left == nil {
		return 1
	}
	if right == nil {
		return -1
	}
	if left.Path != right.Path {
		if left.Path < right.Path {
			return -1
		}
		return 1
	}
	if left.Receiver != right.Receiver {
		if left.Receiver < right.Receiver {
			return -1
		}
		return 1
	}
	if left.Name != right.Name {
		if left.Name < right.Name {
			return -1
		}
		return 1
	}
	if left.LineStart != right.LineStart {
		if left.LineStart < right.LineStart {
			return -1
		}
		return 1
	}
	if left.LineEnd != right.LineEnd {
		if left.LineEnd < right.LineEnd {
			return -1
		}
		return 1
	}
	return 0
}

func compareEventFacts(left *store.EventFact, right *store.EventFact) int {
	if left == nil && right == nil {
		return 0
	}
	if left == nil {
		return 1
	}
	if right == nil {
		return -1
	}
	for _, cmp := range []int{
		compareStrings(left.FuncID, right.FuncID),
		compareInts(left.Line, right.Line),
		compareStrings(left.HandlerRef, right.HandlerRef),
		compareStrings(left.Evidence, right.Evidence),
		compareStrings(left.EventName, right.EventName),
	} {
		if cmp != 0 {
			return cmp
		}
	}
	return 0
}

func compareStrings(left string, right string) int {
	if left == right {
		return 0
	}
	if left < right {
		return -1
	}
	return 1
}

func compareInts(left int, right int) int {
	if left == right {
		return 0
	}
	if left < right {
		return -1
	}
	return 1
}
