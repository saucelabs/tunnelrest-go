package rest

// Protocol allowed to be used.
type Protocol string

// String converts Protocol to string.
func (p *Protocol) String() string {
	return string(*p)
}
