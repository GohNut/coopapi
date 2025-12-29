package handlers

import (
	"bytes"
	"fmt"
	"image/png"
	"net/http"
	"os"
	"time"

	"github.com/fogleman/gg"
	"github.com/labstack/echo/v4"
	"github.com/skip2/go-qrcode"
)

// GenerateSlipHandler generates a slip image and uploads it to R2
func GenerateSlipHandler(c echo.Context) error {
	var req struct {
		SlipInfo struct {
			TransactionRef  string  `json:"transaction_ref"`
			TransactionDate string  `json:"transaction_date"` // Accept as string
			Sender          AccountInfo `json:"sender"`
			Receiver        AccountInfo `json:"receiver"`
			Amount          float64 `json:"amount"`
			QRPayload       string  `json:"qr_payload"`
		} `json:"slip_info"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Invalid request body: %v", err)})
	}

	// Parse transaction date
	txnDate, err := time.Parse(time.RFC3339, req.SlipInfo.TransactionDate)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Invalid transaction_date format: %v", err)})
	}

	slip := SlipInfo{
		TransactionRef:  req.SlipInfo.TransactionRef,
		TransactionDate: txnDate,
		Sender:          req.SlipInfo.Sender,
		Receiver:        req.SlipInfo.Receiver,
		Amount:          req.SlipInfo.Amount,
		QRPayload:       req.SlipInfo.QRPayload,
	}

	// 1. Create Image Context (350x600 is a good size for slip)
	const width = 450
	const height = 750
	dc := gg.NewContext(width, height)

	// Background
	dc.SetRGB(1, 1, 1)
	dc.Clear()

	// 2. Load Fonts
	// Fallback to Arial Unicode if Sarabun not found
	fontPath := "/Users/nutthep/Goh/สหกรณ์/CoopDigital/coopapi/assets/fonts/Sarabun.ttf"
	if _, err := os.Stat(fontPath); os.IsNotExist(err) {
		fontPath = "/System/Library/Fonts/Supplemental/Arial Unicode.ttf"
	}

	// Helper for text rendering
	drawText := func(text string, x, y float64, size float64, colorRGB [3]float64, bold bool) {
		if err := dc.LoadFontFace(fontPath, size); err != nil {
			fmt.Printf("Error loading font: %v\n", err)
		}
		dc.SetRGB(colorRGB[0], colorRGB[1], colorRGB[2])
		dc.DrawString(text, x, y)
	}

	// 3. Draw Slip Content (Simplified version of SlipWidget)
	
	// Circle Icon Placeholder (Green)
	dc.SetRGB255(200, 230, 201) // light green
	dc.DrawCircle(width/2, 80, 40)
	dc.Fill()
	dc.SetRGB255(76, 175, 80) // dark green
	dc.DrawCircle(width/2, 80, 30)
	dc.Fill()
	// Checkmark (simple)
	dc.SetRGB(1, 1, 1)
	dc.SetLineWidth(5)
	dc.DrawLine(width/2-15, 80, width/2, 95)
	dc.DrawLine(width/2, 95, width/2+20, 65)
	dc.Stroke()

	// Success Text
	drawText("โอนเงินสำเร็จ", width/2-60, 160, 28, [3]float64{0.2, 0.6, 0.2}, true)

	// Date
	dateStr := slip.TransactionDate.Format("02 Jan 2006, 15:04")
	drawText(dateStr, width/2-80, 200, 16, [3]float64{0.5, 0.5, 0.5}, false)

	// Amount
	amountStr := fmt.Sprintf("%.2f บาท", slip.Amount)
	drawText(amountStr, width/2-100, 280, 36, [3]float64{0.1, 0.1, 0.1}, true)

	// Divider
	dc.SetRGB(0.9, 0.9, 0.9)
	dc.DrawLine(40, 320, width-40, 320)
	dc.Stroke()

	// Sender Section
	drawText("จาก", 40, 360, 14, [3]float64{0.5, 0.5, 0.5}, false)
	drawText(slip.Sender.Name, 100, 360, 18, [3]float64{0, 0, 0}, true)
	drawText(slip.Sender.AccountNoMasked, 100, 385, 14, [3]float64{0.4, 0.4, 0.4}, false)
	drawText(slip.Sender.BankName, 100, 405, 12, [3]float64{0.6, 0.6, 0.6}, false)

	// Receiver Section
	drawText("ไปยัง", 40, 460, 14, [3]float64{0.5, 0.5, 0.5}, false)
	drawText(slip.Receiver.Name, 100, 460, 18, [3]float64{0, 0, 0}, true)
	drawText(slip.Receiver.AccountNoMasked, 100, 485, 14, [3]float64{0.4, 0.4, 0.4}, false)
	drawText(slip.Receiver.BankName, 100, 505, 12, [3]float64{0.6, 0.6, 0.6}, false)

	// Divider
	dc.SetRGB(0.9, 0.9, 0.9)
	dc.DrawLine(40, 540, width-40, 540)
	dc.Stroke()

	// Ref No
	drawText("เลขที่อ้างอิง", 40, 570, 12, [3]float64{0.5, 0.5, 0.5}, false)
	drawText(slip.TransactionRef, 40, 595, 14, [3]float64{0.2, 0.2, 0.2}, false)

	// QR Code
	if slip.QRPayload != "" {
		qr, err := qrcode.New(slip.QRPayload, qrcode.Medium)
		if err == nil {
			qrImg := qr.Image(120)
			dc.DrawImage(qrImg, width-160, 560)
		}
	}

	// 4. Encode to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to encode image"})
	}

	// 5. Save to Local Storage
	// Create storage directory if not exists
	storageDir := os.Getenv("STORAGE_DIR")
	if storageDir == "" {
		storageDir = "./storage/slips"
	}
	
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Failed to create storage directory: %v", err)})
	}

	filename := fmt.Sprintf("%s_%d.png", slip.TransactionRef, time.Now().Unix())
	filepath := fmt.Sprintf("%s/%s", storageDir, filename)
	
	if err := os.WriteFile(filepath, buf.Bytes(), 0644); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Failed to save file: %v", err)})
	}

	// 6. Return Public URL
	// Read from environment variable for flexibility
	publicUrlBase := os.Getenv("STORAGE_PUBLIC_URL")
	if publicUrlBase == "" {
		publicUrlBase = "https://member.rspcoop.com/storage"
	}
	publicUrl := fmt.Sprintf("%s/slips/%s", publicUrlBase, filename)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "success",
		"url":    publicUrl,
	})
}
