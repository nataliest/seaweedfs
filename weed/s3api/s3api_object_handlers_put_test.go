package s3api

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/seaweedfs/seaweedfs/weed/s3api/s3_constants"
	"github.com/seaweedfs/seaweedfs/weed/s3api/s3err"
	weed_server "github.com/seaweedfs/seaweedfs/weed/server"
	"github.com/seaweedfs/seaweedfs/weed/util/constants"
)

func TestFilerErrorToS3Error(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		expectedErr s3err.ErrorCode
	}{
		{
			name:        "nil error",
			err:         nil,
			expectedErr: s3err.ErrNone,
		},
		{
			name:        "MD5 mismatch error",
			err:         errors.New(constants.ErrMsgBadDigest),
			expectedErr: s3err.ErrBadDigest,
		},
		{
			name:        "Read only error (direct)",
			err:         weed_server.ErrReadOnly,
			expectedErr: s3err.ErrAccessDenied,
		},
		{
			name:        "Read only error (wrapped)",
			err:         fmt.Errorf("create file /buckets/test/file.txt: %w", weed_server.ErrReadOnly),
			expectedErr: s3err.ErrAccessDenied,
		},
		{
			name:        "Context canceled error",
			err:         errors.New("rpc error: code = Canceled desc = context canceled"),
			expectedErr: s3err.ErrInvalidRequest,
		},
		{
			name:        "Context canceled error (simple)",
			err:         errors.New("context canceled"),
			expectedErr: s3err.ErrInvalidRequest,
		},
		{
			name:        "Directory exists error",
			err:         errors.New("existing /path/to/file is a directory"),
			expectedErr: s3err.ErrExistingObjectIsDirectory,
		},
		{
			name:        "File exists error",
			err:         errors.New("/path/to/file is a file"),
			expectedErr: s3err.ErrExistingObjectIsFile,
		},
		{
			name:        "Unknown error",
			err:         errors.New("some random error"),
			expectedErr: s3err.ErrInternalError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filerErrorToS3Error(tt.err)
			if result != tt.expectedErr {
				t.Errorf("filerErrorToS3Error(%v) = %v, want %v", tt.err, result, tt.expectedErr)
			}
		})
	}
}

func TestDetectRequestChecksumAlgorithm(t *testing.T) {
	tests := []struct {
		name              string
		headers           map[string]string
		expectedKey       string
		expectedValue     string
		expectWriter      bool
	}{
		{
			name:         "no checksum",
			headers:      map[string]string{},
			expectedKey:  "",
			expectWriter: false,
		},
		{
			name: "SHA256 header (non-chunked)",
			headers: map[string]string{
				s3_constants.AmzChecksumSHA256: "n4bQgYhMfWWaL+qgxVrQFaO/TxsrC4Is0V1sFbDwCgg=",
			},
			expectedKey:   s3_constants.AmzChecksumSHA256,
			expectedValue: "n4bQgYhMfWWaL+qgxVrQFaO/TxsrC4Is0V1sFbDwCgg=",
			expectWriter:  true,
		},
		{
			name: "CRC32 header (non-chunked)",
			headers: map[string]string{
				s3_constants.AmzChecksumCRC32: "YABb/g==",
			},
			expectedKey:   s3_constants.AmzChecksumCRC32,
			expectedValue: "YABb/g==",
			expectWriter:  true,
		},
		{
			name: "SHA256 trailer (chunked)",
			headers: map[string]string{
				"x-amz-trailer": "x-amz-checksum-sha256",
			},
			expectedKey:   "x-amz-checksum-sha256",
			expectedValue: "",
			expectWriter:  true,
		},
		{
			name: "CRC32C trailer (chunked)",
			headers: map[string]string{
				"x-amz-trailer": "x-amz-checksum-crc32c",
			},
			expectedKey:   "x-amz-checksum-crc32c",
			expectedValue: "",
			expectWriter:  true,
		},
		{
			name: "unsupported trailer",
			headers: map[string]string{
				"x-amz-trailer": "x-amz-checksum-unknown",
			},
			expectedKey:  "",
			expectWriter: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("PUT", "/bucket/key", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			key, value, writer := detectRequestChecksumAlgorithm(req)
			if key != tt.expectedKey {
				t.Errorf("key = %q, want %q", key, tt.expectedKey)
			}
			if value != tt.expectedValue {
				t.Errorf("value = %q, want %q", value, tt.expectedValue)
			}
			if (writer != nil) != tt.expectWriter {
				t.Errorf("writer nil = %v, want nil = %v", writer == nil, !tt.expectWriter)
			}
		})
	}
}
