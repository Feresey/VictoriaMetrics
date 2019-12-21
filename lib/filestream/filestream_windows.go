package filestream

// all fields must be used
var _ = streamTracker{0, 0, 0}

func (st *streamTracker) adviseDontNeed(n int, fdatasync bool) error {
	return nil
}

func (st *streamTracker) close() error {
	return nil
}
