package addon

// Signal defines the signal type
type Signal string

func (s Signal) String() string {
	return string(s)
}
