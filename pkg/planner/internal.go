package planner

type ItemRef struct {
	Path string
	Size int64
}

type Phase1Result struct {
	NewItems     []ItemRef
	DeletedItems []ItemRef
	SizeMismatch []ItemRef
	NeedChecksum []ItemRef
	Identical    []ItemRef
}

type ChecksumData struct {
	ItemRef        ItemRef
	SourceChecksum string
	DestChecksum   string
}
