package go_redisson

type Entry struct {
	GoroutineID map[uint64]int
}

func NewEntry() *Entry {
    return &Entry{
    	GoroutineID: make(map[uint64]int),
	}
}

func (e *Entry) addGoroutineId(goroutineId uint64) {
	count, ok := e.GoroutineID[goroutineId]
	if ok {
		count++
	} else {
		count = 1
	}
	e.GoroutineID[goroutineId] = count
}

func (e *Entry) removeGoroutineId(goroutineId uint64) {
	count, ok := e.GoroutineID[goroutineId]
	if !ok {
		return
	}
	count--
	if count == 0 {
		delete(e.GoroutineID, goroutineId)
	} else {
		e.GoroutineID[goroutineId] = count
	}
}

func (e *Entry) hasNoGoroutine() bool {
	return len(e.GoroutineID) == 0
}
