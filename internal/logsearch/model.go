package logsearch

import (
	"compress/gzip"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	ipv4Pattern     = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	fileDatePattern = regexp.MustCompile(`20\d{2}[-_]?\d{2}[-_]?\d{2}`)
)

type LogRecord struct {
	ID         string
	SourceFile string
	LineNo     int64
	DateKey    int64
	Content    string
	SearchText string
}

type QueryOptions struct {
	DBPath     string
	LogDir     string
	IP         string
	Keyword    string
	StartDate  int64
	EndDate    int64
	SourceLike string
	Limit      int
	ExportPath string
}

func NormalizeIPToken(ip string) string {
	return "ip_" + strings.ReplaceAll(ip, ".", "_")
}

func ExtractIPTokens(line string) []string {
	ips := ipv4Pattern.FindAllString(line, -1)
	if len(ips) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(ips))
	out := make([]string, 0, len(ips))
	for _, ip := range ips {
		token := NormalizeIPToken(ip)
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	return out
}

func BuildSearchText(line string) string {
	tokens := ExtractIPTokens(line)
	if len(tokens) == 0 {
		return line
	}
	return strings.Join(tokens, " ") + " " + line
}

func DateKeyFromFilename(name string) int64 {
	matches := fileDatePattern.FindAllString(name, -1)
	if len(matches) == 0 {
		return 0
	}
	last := matches[len(matches)-1]
	cleaned := strings.NewReplacer("-", "", "_", "").Replace(last)
	value, err := strconv.ParseInt(cleaned, 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func NormalizeDateKey(value string) (int64, error) {
	cleaned := strings.TrimSpace(strings.NewReplacer("-", "", "_", "").Replace(value))
	if cleaned == "" {
		return 0, nil
	}
	switch len(cleaned) {
	case 4:
		cleaned += "0101"
	case 6:
		cleaned += "01"
	case 8:
	default:
		return 0, fmt.Errorf("日期格式不支持：%s", value)
	}
	out, err := strconv.ParseInt(cleaned, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("日期格式不支持：%s", value)
	}
	return out, nil
}

func NormalizeEndDateKey(value string) (int64, error) {
	cleaned := strings.TrimSpace(strings.NewReplacer("-", "", "_", "").Replace(value))
	if cleaned == "" {
		return 0, nil
	}
	switch len(cleaned) {
	case 4:
		cleaned += "1231"
	case 6:
		cleaned += "31"
	case 8:
	default:
		return 0, fmt.Errorf("日期格式不支持：%s", value)
	}
	out, err := strconv.ParseInt(cleaned, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("日期格式不支持：%s", value)
	}
	return out, nil
}

func RecordID(sourceFile string, lineNo int64, content string) string {
	sum := sha1.Sum([]byte(fmt.Sprintf("%s:%d:%s", sourceFile, lineNo, content)))
	return hex.EncodeToString(sum[:])
}

func NewLogRecord(sourcePath string, lineNo int64, content string) LogRecord {
	sourceFile := filepath.Base(sourcePath)
	return LogRecord{
		ID:         RecordID(sourceFile, lineNo, content),
		SourceFile: sourceFile,
		LineNo:     lineNo,
		DateKey:    DateKeyFromFilename(sourceFile),
		Content:    content,
		SearchText: BuildSearchText(content),
	}
}

func OpenMaybeGzip(path string) (io.ReadCloser, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(strings.ToLower(path), ".gz") {
		return file, nil
	}
	reader, err := gzip.NewReader(file)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	return &compoundReadCloser{Reader: reader, closers: []io.Closer{reader, file}}, nil
}

type compoundReadCloser struct {
	io.Reader
	closers []io.Closer
}

func (c *compoundReadCloser) Close() error {
	var first error
	for _, closer := range c.closers {
		if err := closer.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

