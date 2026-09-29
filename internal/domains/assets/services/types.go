package assets_services

// Master describes a stored, normalized image belonging to one consumer's
// (calling service's) tenant namespace.
type Master struct {
	ID       string `json:"id"`
	Consumer string `json:"consumer"`
	Tenant   string `json:"tenant"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Format   string `json:"format"`
	Bytes    int64  `json:"bytes"`
}

// ListedAsset is one entry in a tenant's master listing.
type ListedAsset struct {
	ID           string `json:"id"`
	Bytes        int64  `json:"bytes"`
	LastModified string `json:"last_modified"`
}
