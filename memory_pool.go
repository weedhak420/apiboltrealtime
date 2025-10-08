package main

import (
	"bytes"
	"encoding/json"
	"sync"
)

// MemoryPoolManager manages object pools for better memory management
type MemoryPoolManager struct {
	jsonDecoderPool sync.Pool
	bufferPool      sync.Pool
	jsonEncoderPool sync.Pool
	requestPool     sync.Pool
	responsePool    sync.Pool
}

// NewMemoryPoolManager creates a new memory pool manager
func NewMemoryPoolManager() *MemoryPoolManager {
	return &MemoryPoolManager{
		jsonDecoderPool: sync.Pool{
			New: func() interface{} {
				return json.NewDecoder(nil)
			},
		},
		bufferPool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, 64*1024) // 64KB initial capacity
			},
		},
		jsonEncoderPool: sync.Pool{
			New: func() interface{} {
				return json.NewEncoder(nil)
			},
		},
		requestPool: sync.Pool{
			New: func() interface{} {
				return &bytes.Buffer{}
			},
		},
		responsePool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, 1024*1024) // 1MB initial capacity
			},
		},
	}
}

// GetBuffer gets a buffer from the pool
func (mpm *MemoryPoolManager) GetBuffer() []byte {
	return mpm.bufferPool.Get().([]byte)
}

// PutBuffer returns a buffer to the pool
func (mpm *MemoryPoolManager) PutBuffer(buf []byte) {
	// Reset the slice but keep the underlying array
	buf = buf[:0]
	mpm.bufferPool.Put(buf)
}

// GetRequestBuffer gets a request buffer from the pool
func (mpm *MemoryPoolManager) GetRequestBuffer() *bytes.Buffer {
	buf := mpm.requestPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// PutRequestBuffer returns a request buffer to the pool
func (mpm *MemoryPoolManager) PutRequestBuffer(buf *bytes.Buffer) {
	buf.Reset()
	mpm.requestPool.Put(buf)
}

// GetResponseBuffer gets a response buffer from the pool
func (mpm *MemoryPoolManager) GetResponseBuffer() []byte {
	return mpm.responsePool.Get().([]byte)
}

// PutResponseBuffer returns a response buffer to the pool
func (mpm *MemoryPoolManager) PutResponseBuffer(buf []byte) {
	buf = buf[:0]
	mpm.responsePool.Put(buf)
}

// GetJSONDecoder gets a JSON decoder from the pool
func (mpm *MemoryPoolManager) GetJSONDecoder(reader *bytes.Reader) *json.Decoder {
	decoder := mpm.jsonDecoderPool.Get().(*json.Decoder)
	// Note: We can't reuse the decoder directly, so we create a new one
	// but we can reuse the pool for other purposes
	mpm.jsonDecoderPool.Put(decoder)
	return json.NewDecoder(reader)
}

// GetJSONEncoder gets a JSON encoder from the pool
func (mpm *MemoryPoolManager) GetJSONEncoder(writer *bytes.Buffer) *json.Encoder {
	encoder := mpm.jsonEncoderPool.Get().(*json.Encoder)
	// Note: We can't reuse the encoder directly, so we create a new one
	// but we can reuse the pool for other purposes
	mpm.jsonEncoderPool.Put(encoder)
	return json.NewEncoder(writer)
}

// Global memory pool manager
var memoryPool *MemoryPoolManager

// InitializeMemoryPool initializes the global memory pool
func InitializeMemoryPool() {
	memoryPool = NewMemoryPoolManager()
}
