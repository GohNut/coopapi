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

	// === Image dimensions matching Flutter SlipWidget ===
	const width = 500
	const height = 650
	const paddingX = 30.0
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

	// === Colors (matching Flutter app_colors.dart) ===
	cyanColor := [3]float64{0.0, 0.737, 0.831}       // #00BCD4 - cyan for "โอนเงินสำเร็จ"
	primaryColor := [3]float64{0.102, 0.565, 0.808} // #1A90CE
	successColor := [3]float64{0.298, 0.686, 0.314} // #4CAF50 - green checkmark
	textBlack := [3]float64{0.13, 0.13, 0.13}       // #212121
	textGray := [3]float64{0.5, 0.5, 0.5}           // #808080
	dividerGray := [3]float64{0.85, 0.85, 0.85}     // #D9D9D9

	// === Thai date format ===
	thaiMonths := []string{"ม.ค.", "ก.พ.", "มี.ค.", "เม.ย.", "พ.ค.", "มิ.ย.", "ก.ค.", "ส.ค.", "ก.ย.", "ต.ค.", "พ.ย.", "ธ.ค."}
	thaiYear := slip.TransactionDate.Year() + 543
	dateStr := fmt.Sprintf("%d %s %d, %02d:%02d",
		slip.TransactionDate.Day(),
		thaiMonths[slip.TransactionDate.Month()-1],
		thaiYear,
		slip.TransactionDate.Hour(),
		slip.TransactionDate.Minute())

	// ========== CYAN TOP BAR ==========
	dc.SetRGB(cyanColor[0], cyanColor[1], cyanColor[2])
	dc.DrawRectangle(0, 0, width, 6)
	dc.Fill()

	// ========== HEADER ROW: Logo + Text + Checkmark ==========
	yPos := 35.0
	
	// Load and draw circular logo
	logoPath := "./assets/pic/logoCoop.jpg"
	if _, err := os.Stat(logoPath); os.IsNotExist(err) {
		logoPath = "/app/assets/pic/logoCoop.jpg"
	}
	
	const logoSize = 55
	logoLoaded := false
	if logoFile, err := os.Open(logoPath); err == nil {
		defer logoFile.Close()
		if logoImg, err := jpeg.Decode(logoFile); err == nil {
			resizedLogo := resize.Resize(logoSize, logoSize, logoImg, resize.Lanczos3)
			// Circular mask
			logoCtx := gg.NewContext(logoSize, logoSize)
			logoCtx.DrawCircle(logoSize/2, logoSize/2, logoSize/2)
			logoCtx.Clip()
			logoCtx.DrawImage(resizedLogo, 0, 0)
			dc.DrawImage(logoCtx.Image(), int(paddingX), int(yPos))
			logoLoaded = true
		}
	}
	if !logoLoaded {
		dc.SetRGB(primaryColor[0], primaryColor[1], primaryColor[2])
		dc.DrawCircle(paddingX+logoSize/2, yPos+logoSize/2, logoSize/2)
		dc.Fill()
	}
	
	// "สหกรณ์ รสพ." text next to logo
	drawText("สหกรณ์ รสพ.", paddingX+logoSize+15, yPos+35, 22, cyanColor, true)
	
	// Green checkmark circle on right
	checkX := float64(width) - paddingX - 22
	checkY := yPos + logoSize/2
	// Light green background
	dc.SetRGB255(200, 230, 201)
	dc.DrawCircle(checkX, checkY, 22)
	dc.Fill()
	// Green circle
	dc.SetRGB(successColor[0], successColor[1], successColor[2])
	dc.DrawCircle(checkX, checkY, 18)
	dc.Fill()
	// White checkmark
	dc.SetRGB(1, 1, 1)
	dc.SetLineWidth(4)
	dc.MoveTo(checkX-9, checkY+1)
	dc.LineTo(checkX-2, checkY+8)
	dc.LineTo(checkX+10, checkY-6)
	dc.Stroke()

	// ========== "โอนเงินสำเร็จ" ==========
	yPos += logoSize + 50
	drawTextCentered("โอนเงินสำเร็จ", yPos, 28, cyanColor, true)
	
	// ========== DATE ==========
	yPos += 30
	drawTextCentered(dateStr, yPos, 16, textGray, false)

	// ========== AMOUNT ==========
	yPos += 60
	amountStr := fmt.Sprintf("%.2f บาท", slip.Amount)
	drawTextCentered(amountStr, yPos, 42, textBlack, true)

	// ========== CYAN DIVIDER ==========
	yPos += 40
	dc.SetRGB(cyanColor[0], cyanColor[1], cyanColor[2])
	dc.SetLineWidth(2)
	dc.DrawLine(paddingX, yPos, float64(width)-paddingX, yPos)
	dc.Stroke()

	// ========== SENDER SECTION ==========
	yPos += 45
	drawText("จาก", paddingX, yPos, 18, textGray, false)
	drawText(slip.Sender.Name, paddingX+60, yPos, 20, textBlack, true)
	yPos += 28
	drawText(slip.Sender.AccountNoMasked, paddingX+60, yPos, 16, textGray, false)

	// ========== RECEIVER SECTION ==========
	yPos += 50
	drawText("ไปยัง", paddingX, yPos, 18, textGray, false)
	drawText(slip.Receiver.Name, paddingX+60, yPos, 20, textBlack, true)
	yPos += 28
	drawText(slip.Receiver.AccountNoMasked, paddingX+60, yPos, 16, textGray, false)

	// ========== GRAY DIVIDER ==========
	yPos += 40
	dc.SetRGB(dividerGray[0], dividerGray[1], dividerGray[2])
	dc.SetLineWidth(1)
	dc.DrawLine(paddingX, yPos, float64(width)-paddingX, yPos)
	dc.Stroke()

	// ========== REFERENCE NUMBER ==========
	yPos += 35
	drawText("เลขที่อ้างอิง", paddingX, yPos, 16, textGray, false)
	yPos += 28
	drawText(slip.TransactionRef, paddingX, yPos, 18, textBlack, true)

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
