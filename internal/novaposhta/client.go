package novaposhta

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	apiURL string
	http   *http.Client
}

func NewClient(apiURL string) *Client {
	return &Client{
		apiURL: apiURL,
		http:   &http.Client{Timeout: 30 * time.Second},
	}
}

type npRequest struct {
	APIKey           string         `json:"apiKey"`
	ModelName        string         `json:"modelName"`
	CalledMethod     string         `json:"calledMethod"`
	MethodProperties map[string]any `json:"methodProperties"`
}

type npResponse struct {
	Success bool              `json:"success"`
	Data    []json.RawMessage `json:"data"`
	Errors  []string          `json:"errors"`
}

func (c *Client) call(apiKey, model, method string, props map[string]any) ([]json.RawMessage, error) {
	payload := npRequest{
		APIKey:           apiKey,
		ModelName:        model,
		CalledMethod:     method,
		MethodProperties: props,
	}
	body, _ := json.Marshal(payload)
	resp, err := c.http.Post(c.apiURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("np http: %w", err)
	}
	defer resp.Body.Close()

	var result npResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("np decode: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("np error: %v", result.Errors)
	}
	return result.Data, nil
}

// Document represents a Nova Poshta TTN document.
type Document struct {
	IntDocNumber             string `json:"IntDocNumber"`
	Ref                      string `json:"Ref"`
	ScanSheetNumber          string `json:"ScanSheetNumber"`
	SenderDescription        string `json:"SenderDescription"`
	SenderAddressDescription string `json:"SenderAddressDescription"`
	Sender                   string `json:"Sender"`
	SeatsAmount              any    `json:"SeatsAmount"` // can be string or int
	Printed                  string `json:"Printed"`
	SettlmentAddressData     struct {
		SenderWarehouseRef    string `json:"SenderWarehouseRef"`
		SenderWarehouseNumber string `json:"SenderWarehouseNumber"`
	} `json:"SettlmentAddressData"`
}

type ScanSheet struct {
	Ref         string `json:"Ref"`
	Number      string `json:"Number"`
	Description string `json:"Description"`
}

func (c *Client) GetDocumentInfo(apiKey, ttn string) (*Document, error) {
	data, err := c.call(apiKey, "InternetDocument", "getDocumentList", map[string]any{
		"IntDocNumber": ttn,
		"GetFullList":  "1",
	})
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("document not found")
	}
	var doc Document
	if err := json.Unmarshal(data[0], &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

func (c *Client) GetScanSheetList(apiKey string) ([]ScanSheet, error) {
	data, err := c.call(apiKey, "ScanSheetGeneral", "getScanSheetList", map[string]any{})
	if err != nil {
		return nil, err
	}
	sheets := make([]ScanSheet, 0, len(data))
	for _, d := range data {
		var s ScanSheet
		if err := json.Unmarshal(d, &s); err == nil {
			sheets = append(sheets, s)
		}
	}
	return sheets, nil
}

func (c *Client) GetPrintedDocuments(apiKey string, date time.Time) ([]Document, error) {
	dateStr := date.Format("02.01.2006")
	data, err := c.call(apiKey, "InternetDocument", "getDocumentList", map[string]any{
		"DateTimeFrom": dateStr + " 00:00:00",
		"DateTimeTo":   dateStr + " 23:59:59",
		"GetFullList":  "1",
	})
	if err != nil {
		return nil, err
	}
	docs := make([]Document, 0)
	for _, d := range data {
		var doc Document
		if err := json.Unmarshal(d, &doc); err == nil && doc.Printed == "1" {
			docs = append(docs, doc)
		}
	}
	return docs, nil
}

func (c *Client) InsertDocuments(apiKey string, docRefs []string, sheetRef, description string) error {
	props := map[string]any{
		"DocumentRefs": docRefs,
		"Description":  description,
	}
	if sheetRef != "" {
		props["Ref"] = sheetRef
	}
	_, err := c.call(apiKey, "ScanSheetGeneral", "insertDocuments", props)
	return err
}
