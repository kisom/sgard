package server

import (
	"github.com/kisom/sgard/manifest"
	"github.com/kisom/sgard/sgardpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ManifestToProto converts a manifest.Manifest to its protobuf representation.
func ManifestToProto(m *manifest.Manifest) *sgardpb.Manifest {
	files := make([]*sgardpb.ManifestEntry, len(m.Files))
	for i, e := range m.Files {
		files[i] = EntryToProto(e)
	}
	return &sgardpb.Manifest{
		Version: int32(m.Version),
		Created: timestamppb.New(m.Created),
		Updated: timestamppb.New(m.Updated),
		Message: m.Message,
		Files:   files,
	}
}

// ProtoToManifest converts a protobuf Manifest to a manifest.Manifest.
func ProtoToManifest(p *sgardpb.Manifest) *manifest.Manifest {
	pFiles := p.GetFiles()
	files := make([]manifest.Entry, len(pFiles))
	for i, e := range pFiles {
		files[i] = ProtoToEntry(e)
	}
	return &manifest.Manifest{
		Version: int(p.GetVersion()),
		Created: p.GetCreated().AsTime(),
		Updated: p.GetUpdated().AsTime(),
		Message: p.GetMessage(),
		Files:   files,
	}
}

// EntryToProto converts a manifest.Entry to its protobuf representation.
func EntryToProto(e manifest.Entry) *sgardpb.ManifestEntry {
	return &sgardpb.ManifestEntry{
		Path:    e.Path,
		Hash:    e.Hash,
		Type:    e.Type,
		Mode:    e.Mode,
		Target:  e.Target,
		Updated: timestamppb.New(e.Updated),
	}
}

// ProtoToEntry converts a protobuf ManifestEntry to a manifest.Entry.
func ProtoToEntry(p *sgardpb.ManifestEntry) manifest.Entry {
	return manifest.Entry{
		Path:    p.GetPath(),
		Hash:    p.GetHash(),
		Type:    p.GetType(),
		Mode:    p.GetMode(),
		Target:  p.GetTarget(),
		Updated: p.GetUpdated().AsTime(),
	}
}
