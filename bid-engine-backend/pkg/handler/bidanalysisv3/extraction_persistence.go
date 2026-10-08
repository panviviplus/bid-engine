package bidanalysisv3

import (
	"fmt"
	"regexp"
	"strings"

	"bid-engine/pkg/db/model"
)

var markdownTablePattern = regexp.MustCompile(`(?m)^\s*\|.*\|\s*$`)

func containsMarkdownTable(value string) bool {
	return strings.Contains(value, "|") && markdownTablePattern.MatchString(value)
}

func hasTableEvidence(refs []evidenceRef) bool {
	for _, ref := range refs {
		if ref.Kind == "table" {
			return true
		}
	}
	return false
}

func resolvePacketEvidenceRefs(refs []evidenceRef, packet extractionPacket) []evidenceRef {
	exact := map[string]bool{}
	aliases := map[string][]evidenceRef{}
	for _, block := range packet.Blocks {
		ref := evidenceRef{Kind: "block", Ref: block.Ref}
		exact["block:"+block.Ref] = true
		base := block.Ref
		if offset := strings.LastIndex(base, ":prov:"); offset >= 0 {
			base = base[:offset]
		}
		aliases["block:"+base] = append(aliases["block:"+base], ref)
	}
	for _, table := range packet.Tables {
		ref := evidenceRef{Kind: "table", Ref: table.Ref}
		exact["table:"+table.Ref] = true
		aliases["table:"+table.Ref] = append(aliases["table:"+table.Ref], ref)
	}
	out := make([]evidenceRef, 0, len(refs))
	seen := map[string]bool{}
	appendRef := func(ref evidenceRef) {
		key := ref.Kind + ":" + ref.Ref
		if !seen[key] {
			out = append(out, ref)
			seen[key] = true
		}
	}
	for _, ref := range refs {
		key := ref.Kind + ":" + ref.Ref
		if exact[key] {
			appendRef(ref)
			continue
		}
		if matches := aliases[key]; len(matches) > 0 {
			for _, match := range matches {
				appendRef(match)
			}
			continue
		}
		appendRef(ref)
	}
	return out
}

func normalizeSystemCandidate(candidate *extractedCandidate, specs *extractionSpecs) {
	if candidate == nil || specs == nil || candidate.Origin != "system" {
		return
	}
	if spec, ok := specs.index[candidate.FieldKey]; ok {
		candidate.DisplayName = spec.DisplayName
		candidate.CategoryKey = spec.CategoryKey
		candidate.ValueType = spec.ValueType
	}
}

type evidenceSourceIndex struct {
	blocks map[string]*model.BidAnalysisV3DocumentBlock
	tables map[string]*model.BidAnalysisV3SourceTable
}

func newEvidenceSourceIndex(blocks []*model.BidAnalysisV3DocumentBlock, tables []*model.BidAnalysisV3SourceTable) *evidenceSourceIndex {
	index := &evidenceSourceIndex{blocks: make(map[string]*model.BidAnalysisV3DocumentBlock, len(blocks)), tables: make(map[string]*model.BidAnalysisV3SourceTable, len(tables))}
	for _, block := range blocks {
		index.blocks[block.BlockRef] = block
	}
	for _, table := range tables {
		index.tables[table.TableRef] = table
	}
	return index
}

func buildClauseEvidences(clause *model.BidAnalysisV3Clause, refs []evidenceRef, index *evidenceSourceIndex) ([]*model.BidAnalysisV3ClauseEvidence, error) {
	out := make([]*model.BidAnalysisV3ClauseEvidence, 0, len(refs))
	sortOrder := int32(0)
	for _, ref := range refs {
		if ref.Kind == "block" {
			block := index.blocks[ref.Ref]
			if block == nil {
				return nil, fmt.Errorf("证据 block_ref 不在预加载索引中: %s", ref.Ref)
			}
			out = append(out, &model.BidAnalysisV3ClauseEvidence{
				ProjectID: clause.ProjectID, RunID: clause.RunID, ClauseID: clause.ID,
				SourceKind: "text_block", BlockID: block.ID, PageNo: block.PageNo, Quote: block.Text,
				BboxLeft: block.BboxLeft, BboxTop: block.BboxTop, BboxWidth: block.BboxWidth, BboxHeight: block.BboxHeight, SortOrder: sortOrder,
			})
			sortOrder++
			continue
		}
		table := index.tables[ref.Ref]
		if table == nil {
			return nil, fmt.Errorf("证据 table_ref 不在预加载索引中: %s", ref.Ref)
		}
		for _, region := range sourceTableRegions(table) {
			out = append(out, &model.BidAnalysisV3ClauseEvidence{
				ProjectID: clause.ProjectID, RunID: clause.RunID, ClauseID: clause.ID,
				SourceKind: "source_table", SourceTableID: table.ID, PageNo: region.PageNo, Quote: table.Caption,
				BboxLeft: region.Left, BboxTop: region.Top, BboxWidth: region.Width, BboxHeight: region.Height, SortOrder: sortOrder,
			})
			sortOrder++
		}
	}
	return out, nil
}

func buildFieldEvidences(projectID, runID, valueID int64, refs []evidenceRef, createdBy int64, index *evidenceSourceIndex) ([]*model.BidAnalysisV3FieldValueEvidence, error) {
	out := make([]*model.BidAnalysisV3FieldValueEvidence, 0, len(refs))
	sortOrder := int32(0)
	for _, ref := range refs {
		if ref.Kind == "block" {
			block := index.blocks[ref.Ref]
			if block == nil {
				return nil, fmt.Errorf("证据 block_ref 不在预加载索引中: %s", ref.Ref)
			}
			out = append(out, &model.BidAnalysisV3FieldValueEvidence{
				ProjectID: projectID, RunID: runID, FieldValueID: valueID, SourceKind: "text_block", CreatedBy: createdBy,
				BlockID: block.ID, PageNo: block.PageNo, Quote: block.Text, ContentHash: block.ContentHash,
				BboxLeft: block.BboxLeft, BboxTop: block.BboxTop, BboxWidth: block.BboxWidth, BboxHeight: block.BboxHeight, SortOrder: sortOrder,
			})
			sortOrder++
			continue
		}
		table := index.tables[ref.Ref]
		if table == nil {
			return nil, fmt.Errorf("证据 table_ref 不在预加载索引中: %s", ref.Ref)
		}
		for _, region := range sourceTableRegions(table) {
			out = append(out, &model.BidAnalysisV3FieldValueEvidence{
				ProjectID: projectID, RunID: runID, FieldValueID: valueID, SourceKind: "source_table", CreatedBy: createdBy,
				SourceTableID: table.ID, PageNo: region.PageNo, Quote: table.Caption, ContentHash: table.ContentHash,
				BboxLeft: region.Left, BboxTop: region.Top, BboxWidth: region.Width, BboxHeight: region.Height, SortOrder: sortOrder,
			})
			sortOrder++
		}
	}
	return out, nil
}
