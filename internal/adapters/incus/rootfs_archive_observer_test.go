//go:build linux

package incus

import (
	"encoding/json"
	"strings"
	"testing"
)

func archivePayloadFixture() rootfsStorageSample {
	file := storageByteSample{LogicalBytes: storageMeasurementFileBytes, AllocatedBytes: storageMeasurementFileBytes, ExtentTotalBytes: storageMeasurementFileBytes, ExtentExclusiveBytes: storageMeasurementFileBytes}
	return rootfsStorageSample{Hashes: map[string]string{"base": strings.Repeat("a", 64), "delta": strings.Repeat("b", 64)}, Files: map[string]storageByteSample{"base": file, "delta": file}}
}

func TestArchiveMaterializationNeverInfersPairwiseSharing(t *testing.T) {
	for _, tc := range []struct {
		name                                       string
		beforeShared, sourceShared, importedShared bool
		want                                       string
	}{
		{"exclusive_source_and_import", false, false, false, "separately_materialized_from_source"},
		{"destination_has_other_references", false, false, true, "separately_materialized_from_source"},
		{"shared_rows_do_not_identify_peers", false, true, true, "inconclusive_source_shared"},
		{"shared_source_exclusive_import", false, true, false, "inconclusive_source_shared"},
		{"shared_baseline", true, false, false, "inconclusive_baseline_shared"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before, source, imported := archivePayloadFixture(), archivePayloadFixture(), archivePayloadFixture()
			for i, sample := range []*rootfsStorageSample{&before, &source, &imported} {
				if []bool{tc.beforeShared, tc.sourceShared, tc.importedShared}[i] {
					file := sample.Files["delta"]
					file.ExtentExclusiveBytes = 0
					file.ExtentSetSharedBytes = file.ExtentTotalBytes
					sample.Files["delta"] = file
				}
			}
			got, err := archivePayloadMaterialization(before, source, imported)
			if err != nil || got != tc.want {
				t.Fatalf("relationship=%q error=%v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestArchiveMaterializationRequiresBoundedSamePayload(t *testing.T) {
	for _, change := range []func(*rootfsStorageSample){
		func(s *rootfsStorageSample) { delete(s.Hashes, "delta") },
		func(s *rootfsStorageSample) { s.Hashes["delta"] = strings.Repeat("c", 64) },
		func(s *rootfsStorageSample) { s.Hashes["unexpected"] = strings.Repeat("a", 64) },
		func(s *rootfsStorageSample) { s.Files = nil },
		func(s *rootfsStorageSample) { delete(s.Files, "delta") },
		func(s *rootfsStorageSample) { v := s.Files["delta"]; v.LogicalBytes--; s.Files["delta"] = v },
		func(s *rootfsStorageSample) { v := s.Files["delta"]; v.AllocatedBytes = 0; s.Files["delta"] = v },
		func(s *rootfsStorageSample) { v := s.Files["delta"]; v.ExtentTotalBytes = 0; s.Files["delta"] = v },
	} {
		for i := 0; i < 3; i++ {
			samples := []rootfsStorageSample{archivePayloadFixture(), archivePayloadFixture(), archivePayloadFixture()}
			change(&samples[i])
			if _, err := archivePayloadMaterialization(samples[0], samples[1], samples[2]); err == nil {
				t.Fatal("accepted missing or different payload evidence")
			}
		}
	}
}

func TestRootfsArchiveFileReceiptOnlyContainsFixedRolesAndCounters(t *testing.T) {
	data, err := json.Marshal(archivePayloadFixture())
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, field := range []string{`"payload_files"`, `"base"`, `"delta"`, `"logical_bytes":8388608`, `"extent_exclusive_bytes":8388608`} {
		if !strings.Contains(text, field) {
			t.Fatal("missing fixed measurement field", field)
		}
	}
}
