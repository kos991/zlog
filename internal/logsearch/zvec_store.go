//go:build cgo

package logsearch

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	zvec "github.com/zvec-ai/zvec-go"
)

const (
	fieldID         = "id"
	fieldSourceFile = "source_file"
	fieldLineNo     = "line_no"
	fieldDateKey    = "date_key"
	fieldContent    = "content"
	fieldSearchText = "search_text"
	fieldEmbedding  = "embedding"
)

var dummyVector = []float32{0, 0, 0, 0}

type Store struct {
	collection *zvec.Collection
}

func InitializeZvec() error {
	return zvec.Initialize(nil)
}

func ShutdownZvec() error {
	return zvec.Shutdown()
}

func CreateOrOpenStore(path string) (*Store, error) {
	if _, err := os.Stat(path); err == nil {
		collection, err := zvec.Open(path, nil)
		if err != nil {
			return nil, err
		}
		return &Store{collection: collection}, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	schema, err := buildSchema()
	if err != nil {
		return nil, err
	}
	defer schema.Destroy()

	collection, err := zvec.CreateAndOpen(path, schema, nil)
	if err != nil {
		return nil, err
	}
	return &Store{collection: collection}, nil
}

func (s *Store) Close() error {
	if s == nil || s.collection == nil {
		return nil
	}
	return s.collection.Close()
}

func buildSchema() (*zvec.CollectionSchema, error) {
	schema := zvec.NewCollectionSchema("sangfor_firewall_logs")

	invertParams, err := zvec.NewInvertIndexParams(true, false)
	if err != nil {
		return nil, err
	}
	defer invertParams.Destroy()

	for _, field := range []struct {
		name     string
		dataType zvec.DataType
	}{
		{fieldID, zvec.DataTypeString},
		{fieldSourceFile, zvec.DataTypeString},
		{fieldLineNo, zvec.DataTypeInt64},
		{fieldDateKey, zvec.DataTypeInt64},
		{fieldContent, zvec.DataTypeString},
	} {
		f := zvec.NewFieldSchema(field.name, field.dataType, false, 0)
		if err := f.SetIndexParams(invertParams); err != nil {
			return nil, err
		}
		if err := schema.AddField(f); err != nil {
			return nil, err
		}
	}

	searchField := zvec.NewFieldSchema(fieldSearchText, zvec.DataTypeString, false, 0)
	ftsParams, err := zvec.NewFTSIndexParams("default", nil, "")
	if err != nil {
		return nil, err
	}
	defer ftsParams.Destroy()
	if err := searchField.SetIndexParams(ftsParams); err != nil {
		return nil, err
	}
	if err := schema.AddField(searchField); err != nil {
		return nil, err
	}

	vectorField := zvec.NewFieldSchema(fieldEmbedding, zvec.DataTypeVectorFP32, false, 4)
	hnswParams, err := zvec.NewHNSWIndexParams(zvec.MetricTypeCosine, 16, 200)
	if err != nil {
		return nil, err
	}
	defer hnswParams.Destroy()
	if err := vectorField.SetIndexParams(hnswParams); err != nil {
		return nil, err
	}
	if err := schema.AddField(vectorField); err != nil {
		return nil, err
	}

	return schema, nil
}

func (s *Store) ImportDir(logDir string, batchSize int, out io.Writer) (int, error) {
	paths, err := filepath.Glob(filepath.Join(logDir, "*.log*"))
	if err != nil {
		return 0, err
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return 0, fmt.Errorf("未找到日志文件：%s/*.log*", logDir)
	}
	if batchSize <= 0 {
		batchSize = 1000
	}

	total := 0
	batch := make([]*zvec.Doc, 0, batchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if _, err := s.collection.Upsert(batch); err != nil {
			return err
		}
		for _, doc := range batch {
			doc.Destroy()
		}
		batch = batch[:0]
		return nil
	}

	for _, path := range paths {
		count, err := s.importFile(path, &batch, batchSize, flush)
		if err != nil {
			return total, err
		}
		total += count
		if out != nil {
			fmt.Fprintf(out, "已导入：%s (%d 行)\n", filepath.Base(path), count)
		}
	}
	if err := flush(); err != nil {
		return total, err
	}
	if err := s.collection.Flush(); err != nil {
		return total, err
	}
	return total, nil
}

func (s *Store) importFile(path string, batch *[]*zvec.Doc, batchSize int, flush func() error) (int, error) {
	reader, err := OpenMaybeGzip(path)
	if err != nil {
		return 0, err
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)

	lineNo := int64(0)
	count := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		record := NewLogRecord(path, lineNo, line)
		*batch = append(*batch, recordToDoc(record))
		count++
		if len(*batch) >= batchSize {
			if err := flush(); err != nil {
				return count, err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return count, err
	}
	return count, nil
}

func recordToDoc(record LogRecord) *zvec.Doc {
	doc := zvec.NewDoc()
	doc.SetPK(record.ID)
	_ = doc.AddStringField(fieldID, record.ID)
	_ = doc.AddStringField(fieldSourceFile, record.SourceFile)
	_ = doc.AddInt64Field(fieldLineNo, record.LineNo)
	_ = doc.AddInt64Field(fieldDateKey, record.DateKey)
	_ = doc.AddStringField(fieldContent, record.Content)
	_ = doc.AddStringField(fieldSearchText, record.SearchText)
	_ = doc.AddVectorFP32Field(fieldEmbedding, dummyVector)
	return doc
}

func (s *Store) Query(opts QueryOptions) ([]LogRecord, error) {
	query := zvec.NewSearchQuery()
	defer query.Destroy()
	query.SetFieldName(fieldSearchText)
	query.SetTopK(normalizeLimit(opts.Limit))
	query.SetQueryVector(dummyVector)
	query.SetIncludeVector(false)
	query.SetOutputFields([]string{fieldID, fieldSourceFile, fieldLineNo, fieldDateKey, fieldContent})

	match := buildMatchString(opts)
	if match != "" {
		fts := zvec.NewFTS()
		if err := fts.SetMatchString(match); err != nil {
			fts.Destroy()
			return nil, err
		}
		if err := query.SetFTS(fts); err != nil {
			fts.Destroy()
			return nil, err
		}
		fts.Destroy()
	}
	if filter := buildFilter(opts); filter != "" {
		if err := query.SetFilter(filter); err != nil {
			return nil, err
		}
	}

	docs, err := s.collection.Query(query)
	if err != nil {
		return nil, err
	}
	defer zvec.FreeDocs(docs)

	records := make([]LogRecord, 0, len(docs))
	for _, doc := range docs {
		content, _ := doc.GetStringField(fieldContent)
		if opts.IP != "" && !strings.Contains(content, opts.IP) {
			continue
		}
		if opts.Keyword != "" && !strings.Contains(content, opts.Keyword) {
			continue
		}
		sourceFile, _ := doc.GetStringField(fieldSourceFile)
		if opts.SourceLike != "" && !strings.Contains(sourceFile, opts.SourceLike) {
			continue
		}
		lineNo, _ := doc.GetInt64Field(fieldLineNo)
		dateKey, _ := doc.GetInt64Field(fieldDateKey)
		id, _ := doc.GetStringField(fieldID)
		records = append(records, LogRecord{
			ID:         id,
			SourceFile: sourceFile,
			LineNo:     lineNo,
			DateKey:    dateKey,
			Content:    content,
		})
	}
	return records, nil
}

func buildMatchString(opts QueryOptions) string {
	parts := make([]string, 0, 2)
	if opts.IP != "" {
		parts = append(parts, NormalizeIPToken(opts.IP))
	}
	if opts.Keyword != "" {
		parts = append(parts, opts.Keyword)
	}
	return strings.Join(parts, " ")
}

func buildFilter(opts QueryOptions) string {
	parts := make([]string, 0, 2)
	if opts.StartDate > 0 {
		parts = append(parts, fmt.Sprintf("%s >= %d", fieldDateKey, opts.StartDate))
	}
	if opts.EndDate > 0 {
		parts = append(parts, fmt.Sprintf("%s <= %d", fieldDateKey, opts.EndDate))
	}
	return strings.Join(parts, " AND ")
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 200
	}
	return limit
}

func WriteRecords(records []LogRecord, writer io.Writer) error {
	for _, record := range records {
		if _, err := fmt.Fprintf(writer, "%s:%d:%s\n", record.SourceFile, record.LineNo, record.Content); err != nil {
			return err
		}
	}
	return nil
}

