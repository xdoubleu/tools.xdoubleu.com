package unicat

import "encoding/xml"

// ExternalBook is the normalised representation returned by this package.
// UniCat is a bibliographic catalog and does not carry cover images, so
// CoverURL is intentionally absent.
type ExternalBook struct {
	Title       string
	Authors     []string
	ISBN13      *string
	Description *string
	PageCount   *int
}

// --- SRU/MARCXML response types ---

// sruResponse is the top-level SRU searchRetrieveResponse envelope.
type sruResponse struct {
	XMLName         xml.Name    `xml:"searchRetrieveResponse"`
	NumberOfRecords int         `xml:"numberOfRecords"`
	Records         []sruRecord `xml:"records>record"`
}

// sruRecord is a single SRU result wrapper.
type sruRecord struct {
	RecordData sruRecordData `xml:"recordData"`
}

// sruRecordData wraps the embedded MARC21 record inside the SRU envelope.
type sruRecordData struct {
	MarcRecord marcRecord `xml:"record"`
}

// marcRecord is a MARC21/slim record.
type marcRecord struct {
	DataFields []marcDataField `xml:"datafield"`
}

// marcDataField is a MARC21 variable data field with a tag attribute.
type marcDataField struct {
	Tag       string         `xml:"tag,attr"`
	SubFields []marcSubField `xml:"subfield"`
}

// marcSubField is a MARC21 subfield with a code attribute.
type marcSubField struct {
	Code  string `xml:"code,attr"`
	Value string `xml:",chardata"`
}

// subfieldA returns the value of the first subfield with code "a", or "".
func (df marcDataField) subfieldA() string {
	for _, sf := range df.SubFields {
		if sf.Code == "a" {
			return sf.Value
		}
	}
	return ""
}
