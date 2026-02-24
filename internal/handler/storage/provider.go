package storage

import (
	domainStorage "github.com/glennprays/whatsapp-gateway/domain/storage"
)

// ProvideStorageHandler initializes the storage handler
func ProvideStorageHandler(storage domainStorage.Storage) *StorageHandler {
	return NewStorageHandler(storage)
}
