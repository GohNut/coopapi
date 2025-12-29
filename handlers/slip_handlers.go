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

	// 1. Create Image Context
	const width = 500
	const height = 700
	dc := gg.NewContext(width, height)

	// Background - white
	dc.SetRGB(1, 1, 1)
	dc.Clear()

	// 2. Load Fonts
	fontPath := os.Getenv("FONT_PATH")
	if fontPath == "" {
		fontPath = "/app/assets/fonts/Sarabun-Regular.ttf"
	}
	if _, err := os.Stat(fontPath); os.IsNotExist(err) {
		fontPath = "/System/Library/Fonts/Supplemental/Arial Unicode.ttf"
	}

	// Helper for text rendering
	drawText := func(text string, x, y float64, size float64, colorRGB [3]float64) {
		if err := dc.LoadFontFace(fontPath, size); err != nil {
			fmt.Printf("Error loading font: %v\n", err)
		}
		dc.SetRGB(colorRGB[0], colorRGB[1], colorRGB[2])
		dc.DrawString(text, x, y)
	}

	// Helper for centered text
	drawTextCentered := func(text string, y float64, size float64, colorRGB [3]float64) {
		if err := dc.LoadFontFace(fontPath, size); err != nil {
			fmt.Printf("Error loading font: %v\n", err)
		}
		dc.SetRGB(colorRGB[0], colorRGB[1], colorRGB[2])
		tw, _ := dc.MeasureString(text)
		dc.DrawString(text, (width-tw)/2, y)
	}

	// Thai month names
	thaiMonths := []string{"ม.ค.", "ก.พ.", "มี.ค.", "เม.ย.", "พ.ค.", "มิ.ย.", "ก.ค.", "ส.ค.", "ก.ย.", "ต.ค.", "พ.ย.", "ธ.ค."}
	
	// Convert to Thai Buddhist Era (BE = CE + 543)
	thaiYear := slip.TransactionDate.Year() + 543
	thaiMonth := thaiMonths[slip.TransactionDate.Month()-1]
	dateStr := fmt.Sprintf("%d %s %d, %02d:%02d", 
		slip.TransactionDate.Day(), 
		thaiMonth, 
		thaiYear,
		slip.TransactionDate.Hour(),
		slip.TransactionDate.Minute())

	// === HEADER SECTION ===
	// Cyan top bar
	dc.SetRGB255(0, 188, 212)
	dc.DrawRectangle(0, 0, width, 5)
	dc.Fill()

	// Logo placeholder (circle with text)
	dc.SetRGB255(220, 220, 220)
	dc.DrawCircle(50, 50, 30)
	dc.Fill()
	dc.SetRGB255(0, 150, 136)
	dc.DrawCircle(50, 50, 25)
	dc.Fill()
	drawText("สหกรณ์", 90, 45, 18, [3]float64{0, 0.59, 0.53}) // Teal color
	drawText("สหกรณ์ รสพ.", 90, 65, 16, [3]float64{0.4, 0.4, 0.4})

	// Green checkmark circle (top right)
	dc.SetRGB255(200, 230, 201) // light green
	dc.DrawCircle(width-50, 50, 28)
	dc.Fill()
	dc.SetRGB255(76, 175, 80) // green
	dc.DrawCircle(width-50, 50, 22)
	dc.Fill()
	// Checkmark
	dc.SetRGB(1, 1, 1)
	dc.SetLineWidth(4)
	dc.MoveTo(width-60, 50)
	dc.LineTo(width-50, 60)
	dc.LineTo(width-35, 40)
	dc.Stroke()

	// === SUCCESS TEXT ===
	drawTextCentered("โอนเงินสำเร็จ", 130, 32, [3]float64{0, 0.74, 0.83}) // Cyan color

	// === DATE ===
	drawTextCentered(dateStr, 165, 16, [3]float64{0.5, 0.5, 0.5})

	// === AMOUNT ===
	amountStr := fmt.Sprintf("%.2f บาท", slip.Amount)
	drawTextCentered(amountStr, 230, 40, [3]float64{0.15, 0.15, 0.15})

	// === CYAN DIVIDER ===
	dc.SetRGB255(0, 188, 212)
	dc.SetLineWidth(2)
	dc.DrawLine(40, 260, width-40, 260)
	dc.Stroke()

	// === SENDER SECTION ===
	drawText("จาก", 40, 310, 16, [3]float64{0.5, 0.5, 0.5})
	drawText(slip.Sender.Name, 100, 310, 18, [3]float64{0.1, 0.1, 0.1})
	drawText(slip.Sender.AccountNoMasked, 100, 335, 14, [3]float64{0.4, 0.4, 0.4})

	// === RECEIVER SECTION ===
	drawText("ไปยัง", 40, 400, 16, [3]float64{0.5, 0.5, 0.5})
	drawText(slip.Receiver.Name, 100, 400, 18, [3]float64{0.1, 0.1, 0.1})
	drawText(slip.Receiver.AccountNoMasked, 100, 425, 14, [3]float64{0.4, 0.4, 0.4})

	// === GRAY DIVIDER ===
	dc.SetRGB(0.85, 0.85, 0.85)
	dc.SetLineWidth(1)
	dc.DrawLine(40, 470, width-40, 470)
	dc.Stroke()

	// === REFERENCE NUMBER ===
	drawText("เลขที่อ้างอิง", 40, 510, 14, [3]float64{0.5, 0.5, 0.5})
	drawText(slip.TransactionRef, 40, 535, 16, [3]float64{0.2, 0.2, 0.2})

	// === QR CODE (optional) ===
	if slip.QRPayload != "" {
		qr, err := qrcode.New(slip.QRPayload, qrcode.Medium)
		if err == nil {
			qrImg := qr.Image(100)
			dc.DrawImage(qrImg, width-130, 580)
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
		storageDir = "./storage"
	}
	// Append slips subfolder
	slipsDir := fmt.Sprintf("%s/slips", storageDir)
	
	if err := os.MkdirAll(slipsDir, 0755); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Failed to create storage directory: %v", err)})
	}

	filename := fmt.Sprintf("%s_%d.png", slip.TransactionRef, time.Now().Unix())
	filepath := fmt.Sprintf("%s/%s", slipsDir, filename)
	
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
