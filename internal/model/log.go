package model

import (
	"net/netip"
	"time"
)

type FileMeta struct {
	SourceFile  string
	DeviceIP    netip.Addr
	LogDate     time.Time
	ArchiveDate time.Time
}

type LogRow struct {
	Ts             time.Time
	LogDate        time.Time
	DeviceIP       netip.Addr
	LogType        string
	NatType        string
	SrcIP          netip.Addr
	SrcPort        uint16
	DstIP          netip.Addr
	DstPort        uint16
	Protocol       uint8
	TranslatedIP   netip.Addr
	TranslatedPort uint16
	DstCountry     string
	SourceFile     string
	LineNo         uint32
	ImportedAt     time.Time
}

type ParseError struct {
	SourceFile string
	LineNo     uint32
	Line       string
	Err        error
}

func (e ParseError) Error() string {
	return e.Err.Error()
}
