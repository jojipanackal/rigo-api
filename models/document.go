package models

import (
	"github.com/jojipanackal/rugo/db"
)

type Document struct {
	Id        int64  `db:"id"`
	DocType   string `db:"doc_type"`
	DocFormat string `db:"doc_format"`
	RefId     int64  `db:"ref_id"`
	Filename  string `db:"filename"`
	FileData  []byte `db:"file_data"`
	FileSize  int    `db:"file_size"`
}

type DocumentModel struct{}

// Create stores a new document
func (m *DocumentModel) Create(docType, docFormat string, refId int64, filename string, data []byte) (int64, error) {
	var id int64
	query := `
		INSERT INTO documents (doc_type, doc_format, ref_id, filename, file_data, file_size)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`

	err := db.Instance.QueryRow(query, docType, docFormat, refId, filename, data, len(data)).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// GetByRef retrieves a document by type and reference ID
func (m *DocumentModel) GetByRef(docType string, refId int64) (Document, error) {
	var doc Document
	query := `SELECT id, doc_type, doc_format, ref_id, filename, file_data, file_size FROM documents WHERE doc_type = $1 AND ref_id = $2`
	err := db.Instance.Get(&doc, query, docType, refId)
	return doc, err
}

// Delete removes a document by ID
func (m *DocumentModel) Delete(id int64) error {
	_, err := db.Instance.Exec("DELETE FROM documents WHERE id = $1", id)
	return err
}

// DeleteByRef removes a document by type and reference ID
func (m *DocumentModel) DeleteByRef(docType string, refId int64) error {
	_, err := db.Instance.Exec("DELETE FROM documents WHERE doc_type = $1 AND ref_id = $2", docType, refId)
	return err
}

// Exists checks if a document exists for the given type and reference
func (m *DocumentModel) Exists(docType string, refId int64) bool {
	var count int
	db.Instance.Get(&count, "SELECT COUNT(*) FROM documents WHERE doc_type = $1 AND ref_id = $2", docType, refId)
	return count > 0
}

// Upsert creates or updates a document
func (m *DocumentModel) Upsert(docType, docFormat string, refId int64, filename string, data []byte) error {
	// Delete existing if any
	m.DeleteByRef(docType, refId)

	// Create new
	_, err := m.Create(docType, docFormat, refId, filename, data)
	return err
}
