package download

import (
	"context"
	"net/http"
	"os"

	"hytale-launcher/internal/hytale"
)

// ProgressReport contains information about download progress.
// This is used to send progress updates to a status callback.
type ProgressReport struct {
	// StatusKey is the identifier for this download operation
	StatusKey string

	// Data contains additional key-value data about the download
	Data map[string]any

	// Progress is the overall progress (0.0 to 1.0)
	Progress float64

	// StatusType indicates the type of status update
	StatusType string

	// BytesDownloaded is the number of bytes downloaded so far
	BytesDownloaded int64

	// TotalBytes is the expected total size (-1 if unknown)
	TotalBytes int64

	// Speed is the current download speed in bytes per second
	Speed int64
}

// Reporter creates a ProgressReporter that reports download progress
// through a callback function. It throttles updates to avoid overwhelming
// the UI with too many updates.
//
// Parameters:
//   - statusKey: identifier for this download operation (e.g., "downloading_patch")
//   - data: additional metadata to include in progress reports
//   - progressScale: multiplier for progress (for sub-operations)
//   - progressOffset: offset to add to progress (for sequential operations)
//   - callback: function to receive progress updates
//
// The reported progress will be: progressOffset + (actualProgress * progressScale)
func Reporter(
	statusKey string,
	data map[string]any,
	progressScale float64,
	progressOffset float64,
	callback func(ProgressReport),
) ProgressReporter {
	var (
		lastProgress float64
		lastSpeed    int64
	)

	return func(bytesDownloaded int64, speed int64) {
		// Calculate progress (0.0 to 1.0) within the scale
		var progress float64
		if bytesDownloaded > 0 && speed > 0 {
			// We don't have total size here, so we can't calculate true progress
			// The original code seems to calculate this differently based on content-length
			progress = 0.0
		}

		// Apply scaling
		if progressScale > 0 {
			progress = progress * progressScale
		}

		// Calculate final progress with offset
		finalProgress := progressOffset + progress

		// Throttle updates - only report if progress changed significantly
		// or if speed changed
		shouldReport := shouldReportProgress(lastProgress, finalProgress)
		if !shouldReport && speed == lastSpeed {
			return
		}

		lastProgress = finalProgress
		lastSpeed = speed

		// Send the progress report
		report := ProgressReport{
			StatusKey:       statusKey,
			Data:            data,
			Progress:        finalProgress,
			StatusType:      "update_status",
			BytesDownloaded: bytesDownloaded,
			TotalBytes:      -1, // Unknown
			Speed:           speed,
		}

		callback(report)
	}
}

// shouldReportProgress determines if a progress update should be sent
// based on the change in progress value.
// Updates are throttled to roughly 1% increments, except near 0% and 100%.
func shouldReportProgress(lastProgress, currentProgress float64) bool {
	// Always report at boundaries
	if currentProgress < 0.01 {
		return true
	}
	if currentProgress >= 0.99 {
		return true
	}

	// Report if progress changed by at least 1%
	return currentProgress-lastProgress >= 0.01
}

// StatusReporter is a generic status callback used by pkg package.
type StatusReporter interface{}

// NewReporter creates a reporter adapter for update status reporting.
// This adapts between the pkg.UpdateStatus type and download.ProgressReporter.
func NewReporter(status interface{}, baseProgress, weight float64, callback interface{}) ProgressReporter {
	return func(bytesDownloaded int64, speed int64) {
		// This is a stub adapter - the actual implementation would
		// convert between status types and call the callback
	}
}

// DownloadTempSimple downloads a file to a temp directory and returns the path.
// This is a simplified version that uses default settings.
func DownloadTempSimple(ctx context.Context, url string, reporter ProgressReporter) (string, error) {
	client := http.DefaultClient
	cacheDir := hytale.InStorageDir("cache")

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}

	return DownloadTemp(ctx, client, cacheDir, url, "", reporter)
}

// ReporterWithTotal creates a ProgressReporter that knows the expected total size.
// This allows for accurate progress percentage calculation.
func ReporterWithTotal(
	statusKey string,
	data map[string]any,
	totalBytes int64,
	progressScale float64,
	progressOffset float64,
	callback func(ProgressReport),
) ProgressReporter {
	var (
		lastProgress float64
		lastSpeed    int64
	)

	return func(bytesDownloaded int64, speed int64) {
		// Calculate progress (0.0 to 1.0)
		var progress float64
		if totalBytes > 0 {
			downloaded := bytesDownloaded
			if downloaded > totalBytes {
				downloaded = totalBytes
			}
			progress = float64(downloaded) / float64(totalBytes)
		}

		// Apply scaling
		if progressScale > 0 {
			progress = progress * progressScale
		}

		// Calculate final progress with offset
		finalProgress := progressOffset + progress

		// Throttle updates
		shouldReport := shouldReportProgress(lastProgress, finalProgress)
		if !shouldReport && speed == lastSpeed {
			return
		}

		lastProgress = finalProgress
		lastSpeed = speed

		// Send the progress report
		report := ProgressReport{
			StatusKey:       statusKey,
			Data:            data,
			Progress:        finalProgress,
			StatusType:      "update_status",
			BytesDownloaded: bytesDownloaded,
			TotalBytes:      totalBytes,
			Speed:           speed,
		}

		callback(report)
	}
}
