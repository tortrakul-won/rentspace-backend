package document

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

func renderPDF(ctx context.Context, gotenbergURL string, html []byte) ([]byte, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)

	part, err := mw.CreateFormFile("files", "index.html")
	if err != nil {
		return nil, err
	}
	if _, err = part.Write(html); err != nil {
		return nil, err
	}
	mw.Close()

	url := gotenbergURL + "/forms/chromium/convert/html"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gotenberg: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gotenberg %d: %s", resp.StatusCode, string(b))
	}

	return io.ReadAll(resp.Body)
}
