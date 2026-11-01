package datastore

import (
	"testing"
)

type embeddedMeta struct {
	InternalSecret string `json:"internalSecret" datastore:"-"`
	AuditNotes     string `json:"auditNotes" datastore:",noindex"`
	PublicTag      string `json:"publicTag"`
}

type skippedEmbedded struct {
	LeakedField string `json:"leakedField"`
}

type sampleEntity struct {
	embeddedMeta
	skippedEmbedded `datastore:"-"`
	ID              string `json:"id"`
	Description     string `json:"description" datastore:",noindex"`
	Password        string `json:"password" datastore:"-"`
	PlainField      string
}

func TestSaveStructDatastoreTags(t *testing.T) {
	src := &sampleEntity{
		embeddedMeta: embeddedMeta{
			InternalSecret: "top-secret",
			AuditNotes:     "unindexed-audit-log",
			PublicTag:      "v1",
		},
		skippedEmbedded: skippedEmbedded{
			LeakedField: "must-not-be-saved",
		},
		ID:          "ent-123",
		Description: "large unindexed description",
		Password:    "hunter2",
		PlainField:  "indexed-plain",
	}

	props, err := SaveStruct(src)
	if err != nil {
		t.Fatalf("SaveStruct returned unexpected error: %v", err)
	}

	// Verify PropertyList and PropertyLoadSaver types remain exported and usable
	var list PropertyList = props
	if len(list) == 0 {
		t.Fatalf("expected non-empty PropertyList")
	}

	byName := make(map[string]Property, len(props))
	for _, p := range props {
		byName[p.Name] = p
	}

	if _, exists := byName["password"]; exists {
		t.Errorf("expected field tagged datastore:\"-\" (password) to be omitted")
	}
	if _, exists := byName["internalSecret"]; exists {
		t.Errorf("expected embedded field tagged datastore:\"-\" (internalSecret) to be omitted")
	}
	if _, exists := byName["leakedField"]; exists {
		t.Errorf("expected inner field of embedded struct tagged datastore:\"-\" (leakedField) to be omitted")
	}

	descProp, ok := byName["description"]
	if !ok {
		t.Fatalf("expected description property to be present")
	}
	if !descProp.NoIndex {
		t.Errorf("expected description property to have NoIndex=true")
	}

	auditProp, ok := byName["auditNotes"]
	if !ok {
		t.Fatalf("expected embedded auditNotes property to be present")
	}
	if !auditProp.NoIndex {
		t.Errorf("expected embedded auditNotes property to have NoIndex=true")
	}

	idProp, ok := byName["id"]
	if !ok {
		t.Fatalf("expected id property to be present")
	}
	if idProp.NoIndex {
		t.Errorf("expected id property to have NoIndex=false")
	}

	plainProp, ok := byName["PlainField"]
	if !ok {
		t.Fatalf("expected PlainField property to be present")
	}
	if plainProp.NoIndex {
		t.Errorf("expected PlainField property to have NoIndex=false")
	}

	var dst sampleEntity
	if err := LoadStruct(&dst, props); err != nil {
		t.Fatalf("LoadStruct returned unexpected error: %v", err)
	}
	if dst.ID != src.ID || dst.Description != src.Description || dst.PublicTag != src.PublicTag {
		t.Errorf("LoadStruct round-trip mismatch: got %+v", dst)
	}
	if dst.Password != "" || dst.InternalSecret != "" || dst.LeakedField != "" {
		t.Errorf("expected skipped fields to remain empty after LoadStruct, got %+v", dst)
	}
}
