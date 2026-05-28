package arrowserver

import (
	"fmt"
	"io"

	"github.com/anastasis-tet/lab14/src/go-collector/internal/models"
	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/ipc"
	"github.com/apache/arrow/go/v17/arrow/memory"
)

func WriteAggregates(writer io.Writer, aggregates []models.WindowAggregate) error {
	pool := memory.NewGoAllocator()
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "window_start", Type: arrow.BinaryTypes.String},
		{Name: "window_end", Type: arrow.BinaryTypes.String},
		{Name: "category", Type: arrow.BinaryTypes.String},
		{Name: "count", Type: arrow.PrimitiveTypes.Int64},
		{Name: "min_latitude", Type: arrow.PrimitiveTypes.Float64},
		{Name: "max_latitude", Type: arrow.PrimitiveTypes.Float64},
		{Name: "avg_latitude", Type: arrow.PrimitiveTypes.Float64},
	}, nil)

	starts := array.NewStringBuilder(pool)
	defer starts.Release()
	ends := array.NewStringBuilder(pool)
	defer ends.Release()
	categories := array.NewStringBuilder(pool)
	defer categories.Release()
	counts := array.NewInt64Builder(pool)
	defer counts.Release()
	minLatitudes := array.NewFloat64Builder(pool)
	defer minLatitudes.Release()
	maxLatitudes := array.NewFloat64Builder(pool)
	defer maxLatitudes.Release()
	avgLatitudes := array.NewFloat64Builder(pool)
	defer avgLatitudes.Release()

	for _, item := range aggregates {
		starts.Append(item.WindowStart.Format("2006-01-02T15:04:05Z07:00"))
		ends.Append(item.WindowEnd.Format("2006-01-02T15:04:05Z07:00"))
		categories.Append(item.Category)
		counts.Append(item.Count)
		minLatitudes.Append(item.MinLatitude)
		maxLatitudes.Append(item.MaxLatitude)
		avgLatitudes.Append(item.AvgLatitude)
	}

	columns := []arrow.Array{
		starts.NewArray(),
		ends.NewArray(),
		categories.NewArray(),
		counts.NewArray(),
		minLatitudes.NewArray(),
		maxLatitudes.NewArray(),
		avgLatitudes.NewArray(),
	}
	for _, column := range columns {
		defer column.Release()
	}

	record := array.NewRecord(schema, columns, int64(len(aggregates)))
	defer record.Release()

	ipcWriter := ipc.NewWriter(writer, ipc.WithSchema(schema))
	if err := ipcWriter.Write(record); err != nil {
		_ = ipcWriter.Close()
		return fmt.Errorf("write arrow record: %w", err)
	}
	if err := ipcWriter.Close(); err != nil {
		return fmt.Errorf("close arrow writer: %w", err)
	}
	return nil
}
