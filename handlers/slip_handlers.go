package handlers

import (
	"bytes"
	"fmt"
	"image/jpeg"
	"image/png"
	"net/http"
	"os"
	"time"

	"github.com/fogleman/gg"
	"github.com/labstack/echo/v4"
	"github.com/nfnt/resize"
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

	// 1. Create Image Context - Match Flutter SlipWidget size (350 width)
	const width = 400
	const height = 550
	const padding = 24.0
	dc := gg.NewContext(width, height)

	// Background - white with rounded corners effect
	dc.SetRGB(1, 1, 1)
	dc.Clear()

	// 2. Load Fonts
	fontPath := "./assets/fonts/Sarabun.ttf"
	if _, err := os.Stat(fontPath); os.IsNotExist(err) {
		fontPath = "/app/assets/fonts/Sarabun.ttf"
	}
	if _, err := os.Stat(fontPath); os.IsNotExist(err) {
		fontPath = "/System/Library/Fonts/Supplemental/Arial Unicode.ttf"
	}
	
	boldFontPath := "./assets/fonts/Sarabun Bold.ttf"
	if _, err := os.Stat(boldFontPath); os.IsNotExist(err) {
		boldFontPath = "/app/assets/fonts/Sarabun Bold.ttf"
	}
	if _, err := os.Stat(boldFontPath); os.IsNotExist(err) {
		boldFontPath = fontPath // fallback to regular
	}

	// Helper for text rendering
	drawText := func(text string, x, y float64, size float64, colorRGB [3]float64, bold bool) {
		usePath := fontPath
		if bold {
			usePath = boldFontPath
		}
		if err := dc.LoadFontFace(usePath, size); err != nil {
			dc.LoadFontFace(fontPath, size)
		}
		dc.SetRGB(colorRGB[0], colorRGB[1], colorRGB[2])
		dc.DrawString(text, x, y)
	}

	// Helper for centered text
	drawTextCentered := func(text string, y float64, size float64, colorRGB [3]float64, bold bool) {
		usePath := fontPath
		if bold {
			usePath = boldFontPath
		}
		if err := dc.LoadFontFace(usePath, size); err != nil {
			dc.LoadFontFace(fontPath, size)
		}
		dc.SetRGB(colorRGB[0], colorRGB[1], colorRGB[2])
		tw, _ := dc.MeasureString(text)
		dc.DrawString(text, (width-tw)/2, y)
	}

	// Flutter Theme Colors
	primaryColor := [3]float64{0.102, 0.565, 0.808}  // #1A90CE
	successColor := [3]float64{0.298, 0.686, 0.314}  // #4CAF50
	textPrimaryColor := [3]float64{0.129, 0.129, 0.129}  // #212121
	textSecondaryColor := [3]float64{0.459, 0.459, 0.459}  // #757575
	dividerColor := [3]float64{0.741, 0.741, 0.741}  // #BDBDBD

	// Thai month names
	thaiMonths := []string{"ม.ค.", "ก.พ.", "มี.ค.", "เม.ย.", "พ.ค.", "มิ.ย.", "ก.ค.", "ส.ค.", "ก.ย.", "ต.ค.", "พ.ย.", "ธ.ค."}
	thaiYear := slip.TransactionDate.Year() + 543
	thaiMonth := thaiMonths[slip.TransactionDate.Month()-1]
	dateStr := fmt.Sprintf("%d %s %d, %02d:%02d", 
		slip.TransactionDate.Day(), thaiMonth, thaiYear,
		slip.TransactionDate.Hour(), slip.TransactionDate.Minute())

	// === HEADER: Logo + Text + Checkmark ===
	var yPos float64 = padding
	
	// Load logo as circular image
	logoPath := "./assets/pic/logoCoop.jpg"
	if _, err := os.Stat(logoPath); os.IsNotExist(err) {
		logoPath = "/app/assets/pic/logoCoop.jpg"
	}
	
	const logoSize = 45
	logoLoaded := false
	if logoFile, err := os.Open(logoPath); err == nil {
		defer logoFile.Close()
		if logoImg, err := jpeg.Decode(logoFile); err == nil {
			// Resize to square first
			resizedLogo := resize.Resize(logoSize, logoSize, logoImg, resize.Lanczos3)
			
			// Create circular mask
			logoContext := gg.NewContext(logoSize, logoSize)
			logoContext.DrawCircle(logoSize/2, logoSize/2, logoSize/2)
			logoContext.Clip()
			logoContext.DrawImage(resizedLogo, 0, 0)
			
			dc.DrawImage(logoContext.Image(), int(padding), int(yPos))
			logoLoaded = true
		}
	}
	
	if !logoLoaded {
		dc.SetRGB(primaryColor[0], primaryColor[1], primaryColor[2])
		dc.DrawCircle(padding+logoSize/2, yPos+logoSize/2, logoSize/2)
		dc.Fill()
	}
	
	// "สหกรณ์ รสพ." text
	drawText("สหกรณ์ รสพ.", padding+logoSize+12, yPos+30, 18, primaryColor, true)
	
	// Checkmark icon (right side)
	checkX := float64(width) - padding - 16
	checkY := yPos + logoSize/2
	dc.SetRGB(successColor[0], successColor[1], successColor[2])
	dc.DrawCircle(checkX, checkY, 16)
	dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.SetLineWidth(3)
	dc.MoveTo(checkX-8, checkY)
	dc.LineTo(checkX-2, checkY+6)
	dc.LineTo(checkX+8, checkY-6)
	dc.Stroke()

	// === SUCCESS TEXT ===
	yPos += logoSize + 30
	drawTextCentered("โอนเงินสำเร็จ", yPos, 22, successColor, true)
	
	// === DATE ===
	yPos += 25
	drawTextCentered(dateStr, yPos, 14, textSecondaryColor, false)

	// === AMOUNT (Large, Bold) ===
	yPos += 45
	amountStr := fmt.Sprintf("%.2f บาท", slip.Amount)
	drawTextCentered(amountStr, yPos, 36, textPrimaryColor, true)

	// === DIVIDER ===
	yPos += 30
	dc.SetRGB(dividerColor[0], dividerColor[1], dividerColor[2])
	dc.SetLineWidth(1)
	dc.DrawLine(padding, yPos, float64(width)-padding, yPos)
	dc.Stroke()

	// === SENDER SECTION ===
	yPos += 35
	drawText("จาก", padding, yPos, 14, textSecondaryColor, false)
	drawText(slip.Sender.Name, padding+50, yPos, 16, textPrimaryColor, true)
	yPos += 22
	drawText(slip.Sender.AccountNoMasked, padding+50, yPos, 14, textSecondaryColor, false)

	// === RECEIVER SECTION ===
	yPos += 40
	drawText("ไปยัง", padding, yPos, 14, textSecondaryColor, false)
	drawText(slip.Receiver.Name, padding+50, yPos, 16, textPrimaryColor, true)
	yPos += 22
	drawText(slip.Receiver.AccountNoMasked, padding+50, yPos, 14, textSecondaryColor, false)

	// === DIVIDER ===
	yPos += 30
	dc.SetRGB(dividerColor[0], dividerColor[1], dividerColor[2])
	dc.DrawLine(padding, yPos, float64(width)-padding, yPos)
	dc.Stroke()

	// === REFERENCE NUMBER ===
	yPos += 25
	drawText("เลขที่อ้างอิง", padding, yPos, 13, textSecondaryColor, false)
	yPos += 20
	drawText(slip.TransactionRef, padding, yPos, 14, textPrimaryColor, true)

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
