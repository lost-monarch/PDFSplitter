package main

import (
	"fmt"
	"image/png"
	"log"
	"os"
	"os/exec"        // <-- needed for exec.Command
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"io/ioutil"       // <-- needed for ioutil.TempDir

	pdfium "github.com/klippa-app/go-pdfium"
)

// Path to scan PDFs
var scansPath = `F:\NFI\Printers\Canon 5540\Oscar`

// Extract quotation number from text
func extractNumber(text string) string {
	re := regexp.MustCompile(`Quotation No\.\s*:\s*(\w+)`)
	match := re.FindStringSubmatch(text)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

// Extract version from text
func extractVersion(text string) string {
	re := regexp.MustCompile(`Version\s*:\s*(\S+)`)
	match := re.FindStringSubmatch(text)
	if len(match) > 1 {
		return match[1]
	}
	return "1"
}

// Run Tesseract OCR on an image
func ocrImage(imgPath string) (string, error) {
	cmd := exec.Command(
		`C:\Users\research2\Desktop\Python-3.13.9\Projects\PDFSplitter\tesseract-4.1.1\tesseract.exe`,
		imgPath, "stdout",
	)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tesseract failed: %w", err)
	}
	return string(out), nil
}

// Determine page type
func pageType(text string) string {
	if strings.Contains(text, "Certificate of Analysis") || strings.Contains(text, "Specification Sheet") {
		return "CoA"
	} else if strings.Contains(text, "PILOT") {
		return "Pilot"
	}
	return "Unknown"
}

func splitPDF(pdfPath string, wg *sync.WaitGroup) {
	defer wg.Done()
	fmt.Println("Processing:", pdfPath)

	// Create PDFium instance
	pdf, err := pdfium.NewPdfium()
	if err != nil {
		log.Println("Failed to create PDFium instance:", err)
		return
	}
	defer pdf.Close()

	doc, err := pdf.NewDocument(pdfPath)
	if err != nil {
		log.Println("Failed to open PDF:", err)
		return
	}
	defer doc.Close()

	numPages, err := doc.GetPageCount()
	if err != nil {
		log.Println("Failed to get page count:", err)
		return
	}
	fmt.Printf("PDF has %d pages\n", numPages)

	// Temporary directory for images
	tmpDir := os.TempDir()
	texts := make([]string, numPages)

	for i := 0; i < numPages; i++ {
		page, err := doc.GetPage(i)
		if err != nil {
			log.Println("Failed to get page:", err)
			return
		}

		img, err := page.RenderImage(300, 300) // DPI
		if err != nil {
			log.Println("Failed to render page:", err)
			return
		}

		imgPath := filepath.Join(tmpDir, fmt.Sprintf("page_%d.png", i+1))
		outFile, _ := os.Create(imgPath)
		png.Encode(outFile, img)
		outFile.Close()

		text, err := ocrImage(imgPath)
		if err != nil {
			log.Println("OCR failed:", err)
			return
		}
		texts[i] = text
	}

	// Detect CoA pages
	var hPages []int
	for i, t := range texts {
		if pageType(t) == "CoA" {
			hPages = append(hPages, i)
		}
	}

	// Detect Pilots
	pPages := make(map[[2]string][]int)
	for i, t := range texts {
		number := extractNumber(t)
		if number != "" {
			version := extractVersion(t)
			key := [2]string{number, version}
			pPages[key] = append(pPages[key], i)
		}
	}

	// Create splits folder
	scriptDir, _ := os.Getwd()
	coaDir := filepath.Join(scriptDir, "splits")
	os.MkdirAll(coaDir, os.ModePerm)

	// Save CoA PDFs
	for idx, pageIndex := range coaPages {
		outDoc, _ := pdf.NewEmptyDocument()
		page, _ := doc.GetPage(pageIndex)
		outDoc.AddPage(page)

		outFile := filepath.Join(coaDir, fmt.Sprintf("CoA_%d.pdf", idx+1))
		outDoc.WriteToFile(outFile)
		outDoc.Close()
	}

	// Save Pilot PDFs
	for key, pages := range pilotPages {
		num := key[0]
		ver := key[1]
		outDir := filepath.Join(`F:\NFI\RID\Formulation\R&D Pilots\Pilots`, "QB-"+num)
		os.MkdirAll(outDir, os.ModePerm)

		outFile := filepath.Join(outDir, fmt.Sprintf("PilotReport_V%s.pdf", ver))
		if _, err := os.Stat(outFile); !os.IsNotExist(err) {
			log.Printf("File exists, skipping: %s", outFile)
			continue
		}

		outDoc, _ := pdf.NewEmptyDocument()
		for _, pageIndex := range pages {
			page, _ := doc.GetPage(pageIndex)
			outDoc.AddPage(page)
		}
		outDoc.WriteToFile(outFile)
		outDoc.Close()
	}

	fmt.Println("Done:", pdfPath)
}


func main() {
	files, err := ioutil.ReadDir(scansPath)
	if err != nil {
		log.Fatal(err)
	}

	var pdfs []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(strings.ToLower(f.Name()), ".pdf") {
			pdfs = append(pdfs, filepath.Join(scansPath, f.Name()))
		}
	}

	var wg sync.WaitGroup
	for _, pdf := range pdfs {
		wg.Add(1)
		go splitPDF(pdf, &wg)
	}
	wg.Wait()
	fmt.Println("All PDFs processed.")
}
