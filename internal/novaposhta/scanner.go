package novaposhta

import (
	"fmt"
	"strconv"
	"time"
)

type TTNStatus string

const (
	StatusOK                TTNStatus = "ok"
	StatusNotFound          TTNStatus = "not_found"
	StatusAlreadyInRegistry TTNStatus = "already_in_registry"
	StatusDuplicate         TTNStatus = "duplicate"
	StatusError             TTNStatus = "error"
)

type ValidateResult struct {
	TTN     string    `json:"ttn"`
	Status  TTNStatus `json:"status"`
	Message string    `json:"message,omitempty"`
	DocRef  string    `json:"doc_ref,omitempty"`
	// Fields for grouping
	SenderRef                string `json:"-"`
	WarehouseRef             string `json:"-"`
	SenderDescription        string `json:"sender_description,omitempty"`
	SenderAddressDescription string `json:"sender_address_description,omitempty"`
	WarehouseNumber          string `json:"warehouse_number,omitempty"`
	ScanSheetNumber          string `json:"scan_sheet_number,omitempty"`
}

type Group struct {
	Key                      string   `json:"key"`
	SuggestedName            string   `json:"suggested_name"`
	SenderDescription        string   `json:"sender_description"`
	SenderAddressDescription string   `json:"sender_address_description"`
	WarehouseNumber          string   `json:"warehouse_number"`
	TTNs                     []string `json:"ttns"`
	DocRefs                  []string `json:"doc_refs"`
	TTNCount                 int      `json:"ttn_count"`
}

type DistributeResult struct {
	TTN      string `json:"ttn"`
	Status   string `json:"status"`
	Message  string `json:"message,omitempty"`
	Registry string `json:"registry,omitempty"`
}

// ValidateTTN checks a single TTN via the NP API.
func ValidateTTN(client *Client, apiKey, ttn string) ValidateResult {
	doc, err := client.GetDocumentInfo(apiKey, ttn)
	if err != nil {
		return ValidateResult{TTN: ttn, Status: StatusNotFound}
	}
	if doc.ScanSheetNumber != "" {
		return ValidateResult{
			TTN:             ttn,
			Status:          StatusAlreadyInRegistry,
			ScanSheetNumber: doc.ScanSheetNumber,
			Message:         fmt.Sprintf("вже у реєстрі %s", doc.ScanSheetNumber),
		}
	}
	return ValidateResult{
		TTN:                      ttn,
		Status:                   StatusOK,
		DocRef:                   doc.Ref,
		SenderRef:                doc.Sender,
		WarehouseRef:             doc.SettlmentAddressData.SenderWarehouseRef,
		SenderDescription:        doc.SenderDescription,
		SenderAddressDescription: doc.SenderAddressDescription,
		WarehouseNumber:          doc.SettlmentAddressData.SenderWarehouseNumber,
	}
}

// seatsAmount extracts numeric seats count from the mixed-type field.
func seatsAmount(doc *Document) int {
	switch v := doc.SeatsAmount.(type) {
	case float64:
		return int(v)
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 1
}

// GroupResults groups validated OK results by (sender, warehouse).
func GroupResults(results []ValidateResult) []Group {
	type entry struct {
		group Group
	}
	index := map[string]*Group{}
	order := []string{}

	for _, r := range results {
		if r.Status != StatusOK {
			continue
		}
		key := r.SenderRef + "|" + r.WarehouseRef
		if _, ok := index[key]; !ok {
			name := fmt.Sprintf("%s_%s_ВД%s",
				r.SenderDescription,
				time.Now().Format("2006.02.01"),
				r.WarehouseNumber,
			)
			index[key] = &Group{
				Key:                      key,
				SuggestedName:            name,
				SenderDescription:        r.SenderDescription,
				SenderAddressDescription: r.SenderAddressDescription,
				WarehouseNumber:          r.WarehouseNumber,
			}
			order = append(order, key)
		}
		g := index[key]
		g.TTNs = append(g.TTNs, r.TTN)
		g.DocRefs = append(g.DocRefs, r.DocRef)
		g.TTNCount++
	}

	groups := make([]Group, 0, len(order))
	for _, k := range order {
		groups = append(groups, *index[k])
	}
	return groups
}

type DistributeInput struct {
	Key       string   `json:"key"`
	DocRefs   []string `json:"doc_refs"`
	TTNs      []string `json:"ttns"`
	SheetName string   `json:"sheet_name"`
}

// Distribute creates or updates scan sheets for the given groups.
func Distribute(client *Client, apiKey string, inputs []DistributeInput) []DistributeResult {
	// Load existing sheets once
	existingSheets, _ := client.GetScanSheetList(apiKey)
	sheetByName := map[string]string{} // name → ref
	for _, s := range existingSheets {
		sheetByName[s.Description] = s.Ref
	}

	var results []DistributeResult
	for _, inp := range inputs {
		ref := sheetByName[inp.SheetName]
		err := client.InsertDocuments(apiKey, inp.DocRefs, ref, inp.SheetName)
		status := "done"
		msg := ""
		if err != nil {
			status = "error"
			msg = err.Error()
		}
		for _, ttn := range inp.TTNs {
			results = append(results, DistributeResult{
				TTN:      ttn,
				Status:   status,
				Message:  msg,
				Registry: inp.SheetName,
			})
		}
	}
	return results
}
