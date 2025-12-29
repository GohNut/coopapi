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

	// === Image dimensions per slip_layout_spec.md ===
	const width = 350
	const height = 450  // Auto height ~420-450px
	const paddingH = 20.0  // Horizontal padding
	const paddingV = 24.0  // Vertical padding
	dc := gg.NewContext(width, height)

	// Background - white
	dc.SetRGB(1, 1, 1)
	dc.Clear()

	// === Load Fonts ===
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
		boldFontPath = fontPath
	}

	// Text rendering helpers
	drawText := func(text string, x, y float64, size float64, colorRGB [3]float64, bold bool) {
		usePath := fontPath
		if bold {
			usePath = boldFontPath
		}
		dc.LoadFontFace(usePath, size)
		dc.SetRGB(colorRGB[0], colorRGB[1], colorRGB[2])
		dc.DrawString(text, x, y)
	}

	drawTextCentered := func(text string, y float64, size float64, colorRGB [3]float64, bold bool) {
		usePath := fontPath
		if bold {
			usePath = boldFontPath
		}
		dc.LoadFontFace(usePath, size)
		dc.SetRGB(colorRGB[0], colorRGB[1], colorRGB[2])
		tw, _ := dc.MeasureString(text)
		dc.DrawString(text, (width-tw)/2, y)
	}

	// === Colors per slip_layout_spec.md ===
	primaryGreen := [3]float64{0.0, 0.424, 0.278}   // #006C47 - สีเขียวสหกรณ์
	successGreen := [3]float64{0.0, 0.784, 0.325}   // #00C853 - สีเขียวสำเร็จ
	textPrimary := [3]float64{0.129, 0.129, 0.129}  // #212121 - ดำ
	textSecondary := [3]float64{0.459, 0.459, 0.459} // #757575 - เทา
	dividerColor := [3]float64{0.878, 0.878, 0.878}  // #E0E0E0 - เทาอ่อน

	// === Thai date format ===
	thaiMonths := []string{"ม.ค.", "ก.พ.", "มี.ค.", "เม.ย.", "พ.ค.", "มิ.ย.", "ก.ค.", "ส.ค.", "ก.ย.", "ต.ค.", "พ.ย.", "ธ.ค."}
	thaiYear := slip.TransactionDate.Year() + 543
	dateStr := fmt.Sprintf("%d %s %d, %02d:%02d",
		slip.TransactionDate.Day(),
		thaiMonths[slip.TransactionDate.Month()-1],
		thaiYear,
		slip.TransactionDate.Hour(),
		slip.TransactionDate.Minute())

	// === HEADER ROW: Logo + Text + Checkmark ===
	yPos := paddingV
	
	// Load and draw circular logo (45x45 per spec)
	logoPath := "./assets/pic/logoCoop.jpg"
	if _, err := os.Stat(logoPath); os.IsNotExist(err) {
		logoPath = "/app/assets/pic/logoCoop.jpg"
	}
	
	const logoSize = 45
	logoLoaded := false
	if logoFile, err := os.Open(logoPath); err == nil {
		defer logoFile.Close()
		if logoImg, err := jpeg.Decode(logoFile); err == nil {
			// Resize to exact square size
			resizedLogo := resize.Resize(logoSize, logoSize, logoImg, resize.Lanczos3)
			// Create circular mask
			logoCtx := gg.NewContext(logoSize, logoSize)
			logoCtx.DrawCircle(float64(logoSize)/2.0, float64(logoSize)/2.0, float64(logoSize)/2.0)
			logoCtx.Clip()
			logoCtx.DrawImage(resizedLogo, 0, 0)
			dc.DrawImage(logoCtx.Image(), int(paddingH), int(yPos))
			logoLoaded = true
		}
	}
	if !logoLoaded {
		dc.SetRGB(primaryGreen[0], primaryGreen[1], primaryGreen[2])
		dc.DrawCircle(paddingH+float64(logoSize)/2.0, yPos+float64(logoSize)/2.0, float64(logoSize)/2.0)
		dc.Fill()
	}
	
	// "สหกรณ์ รสพ." text - 18pt Bold, primary color
	drawText("สหกรณ์ รสพ.", paddingH+float64(logoSize)+12, yPos+28, 18, primaryGreen, true)
	
	// Success checkmark (32x32 per spec)
	checkX := float64(width) - paddingH - 16
	checkY := yPos + float64(logoSize)/2.0
	// Green circle for success
	dc.SetRGB(successGreen[0], successGreen[1], successGreen[2])
	dc.DrawCircle(checkX, checkY, 16)
	dc.Fill()
	// White checkmark
	dc.SetRGB(1, 1, 1)
	dc.SetLineWidth(3)
	dc.MoveTo(checkX-7, checkY)
	dc.LineTo(checkX-2, checkY+5)
	dc.LineTo(checkX+7, checkY-5)
	dc.Stroke()

	// === "โอนเงินสำเร็จ" - 20pt Bold, success color ===
	yPos += float64(logoSize) + 24
	drawTextCentered("โอนเงินสำเร็จ", yPos, 20, successGreen, true)
	
	// === DATE - 13pt Regular, textSecondary ===
	yPos += 20
	drawTextCentered(dateStr, yPos, 13, textSecondary, false)

	// === AMOUNT - 32pt Bold, textPrimary ===
	yPos += 40
	amountStr := fmt.Sprintf("%.2f บาท", slip.Amount)
	drawTextCentered(amountStr, yPos, 32, textPrimary, true)

	// === DIVIDER - #E0E0E0 ===
	yPos += 24
	dc.SetRGB(dividerColor[0], dividerColor[1], dividerColor[2])
	dc.SetLineWidth(1)
	dc.DrawLine(paddingH, yPos, float64(width)-paddingH, yPos)
	dc.Stroke()

	// === SENDER SECTION ===
	yPos += 24
	// "จาก" - 14pt Regular, textSecondary, fixed 40px width
	drawText("จาก", paddingH, yPos, 14, textSecondary, false)
	// Account name - 15pt Bold
	drawText(slip.Sender.Name, paddingH+48, yPos, 15, textPrimary, true)
	// Account number - 13pt Regular, textSecondary
	yPos += 18
	drawText(slip.Sender.AccountNoMasked, paddingH+48, yPos, 13, textSecondary, false)

	// === RECEIVER SECTION ===
	yPos += 28
	// "ไปยัง" - 14pt Regular, textSecondary
	drawText("ไปยัง", paddingH, yPos, 14, textSecondary, false)
	// Account name - 15pt Bold
	drawText(slip.Receiver.Name, paddingH+48, yPos, 15, textPrimary, true)
	// Account number - 13pt Regular, textSecondary
	yPos += 18
	drawText(slip.Receiver.AccountNoMasked, paddingH+48, yPos, 13, textSecondary, false)

	// === DIVIDER ===
	yPos += 24
	dc.SetRGB(dividerColor[0], dividerColor[1], dividerColor[2])
	dc.DrawLine(paddingH, yPos, float64(width)-paddingH, yPos)
	dc.Stroke()

	// === REFERENCE NUMBER ===
	yPos += 20
	// "เลขที่อ้างอิง" - 12pt Regular, textSecondary
	drawText("เลขที่อ้างอิง", paddingH, yPos, 12, textSecondary, false)
	// Ref value - 13pt Medium
	yPos += 16
	drawText(slip.TransactionRef, paddingH, yPos, 13, textPrimary, true)

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
