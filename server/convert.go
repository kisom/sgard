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
	pb := &sgardpb.Manifest{
		Version: int32(m.Version),
		Created: timestamppb.New(m.Created),
		Updated: timestamppb.New(m.Updated),
		Message: m.Message,
		Files:   files,
	}
	if m.Encryption != nil {
		pb.Encryption = EncryptionToProto(m.Encryption)
	}
	return pb
}

// ProtoToManifest converts a protobuf Manifest to a manifest.Manifest.
func ProtoToManifest(p *sgardpb.Manifest) *manifest.Manifest {
	pFiles := p.GetFiles()
	files := make([]manifest.Entry, len(pFiles))
	for i, e := range pFiles {
		files[i] = ProtoToEntry(e)
	}
	m := &manifest.Manifest{
		Version: int(p.GetVersion()),
		Created: p.GetCreated().AsTime(),
		Updated: p.GetUpdated().AsTime(),
		Message: p.GetMessage(),
		Files:   files,
	}
	if p.GetEncryption() != nil {
		m.Encryption = ProtoToEncryption(p.GetEncryption())
	}
	return m
}

// EntryToProto converts a manifest.Entry to its protobuf representation.
func EntryToProto(e manifest.Entry) *sgardpb.ManifestEntry {
	return &sgardpb.ManifestEntry{
		Path:          e.Path,
		Hash:          e.Hash,
		Type:          e.Type,
		Mode:          e.Mode,
		Target:        e.Target,
		Updated:       timestamppb.New(e.Updated),
		PlaintextHash: e.PlaintextHash,
		Encrypted:     e.Encrypted,
	}
}

// ProtoToEntry converts a protobuf ManifestEntry to a manifest.Entry.
func ProtoToEntry(p *sgardpb.ManifestEntry) manifest.Entry {
	return manifest.Entry{
		Path:          p.GetPath(),
		Hash:          p.GetHash(),
		Type:          p.GetType(),
		Mode:          p.GetMode(),
		Target:        p.GetTarget(),
		Updated:       p.GetUpdated().AsTime(),
		PlaintextHash: p.GetPlaintextHash(),
		Encrypted:     p.GetEncrypted(),
	}
}

// EncryptionToProto converts a manifest.Encryption to its protobuf representation.
func EncryptionToProto(e *manifest.Encryption) *sgardpb.Encryption {
	slots := make(map[string]*sgardpb.KekSlot, len(e.KekSlots))
	for name, slot := range e.KekSlots {
		slots[name] = &sgardpb.KekSlot{
			Type:          slot.Type,
			Argon2Time:    int32(slot.Argon2Time),
			Argon2Memory:  int32(slot.Argon2Memory),
			Argon2Threads: int32(slot.Argon2Threads),
			CredentialId:  slot.CredentialID,
			Salt:          slot.Salt,
			WrappedDek:    slot.WrappedDEK,
		}
	}
	return &sgardpb.Encryption{
		Algorithm: e.Algorithm,
		KekSlots:  slots,
	}
}

// ProtoToEncryption converts a protobuf Encryption to a manifest.Encryption.
func ProtoToEncryption(p *sgardpb.Encryption) *manifest.Encryption {
	slots := make(map[string]*manifest.KekSlot, len(p.GetKekSlots()))
	for name, slot := range p.GetKekSlots() {
		slots[name] = &manifest.KekSlot{
			Type:          slot.GetType(),
			Argon2Time:    int(slot.GetArgon2Time()),
			Argon2Memory:  int(slot.GetArgon2Memory()),
			Argon2Threads: int(slot.GetArgon2Threads()),
			CredentialID:  slot.GetCredentialId(),
			Salt:          slot.GetSalt(),
			WrappedDEK:    slot.GetWrappedDek(),
		}
	}
	return &manifest.Encryption{
		Algorithm: p.GetAlgorithm(),
		KekSlots:  slots,
	}
}
