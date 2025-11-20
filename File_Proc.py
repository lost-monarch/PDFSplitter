import os
import re
import pytesseract
from concurrent.futures import ProcessPoolExecutor
from PyPDF2 import PdfReader, PdfWriter
from pdf2image import convert_from_path

scans_path = os.path.abspath("F:\\NFI\\Printers\\Canon 5540\\Oscar") 

def ocr_all_pages(images):
    texts = []
    for img in images:
        text = pytesseract.image_to_string(img)
        texts.append(text)
    return texts

def extract_number(text):
    text = text.strip()
    match = re.search(r"Quotation No\.\s*:\s*(\w+)", text)
    if match:
        return match.group(1)

def page_type(texts):
    page_types = {}

    header_phrases = ["Certificate of Analysis", "Specification Sheet"]
    pilot_keywords = ["PILOT"]

    for i, text in enumerate(texts):
        if any(h in text for h in header_phrases):
            page_types[i] = "CoA"
        elif any(k in text for k in pilot_keywords):
            page_types[i] = "Pilot"
        else:
            page_types[i] = "Unknown"

    return page_types

def coa_detection(texts):
    header_phrases = ["Certificate of Analysis", "Specification Sheet"]
    return [
        i for i, text in enumerate(texts)
        if any(h in text for h in header_phrases)
    ]

def version_detection(text):
    text = text.strip()
    match = re.search(r"Version\s*:\s*(\S+)", text)  # \S+ = non-whitespace
    if match:
        return match.group(1)
    return None

def pilot_detection(texts):

    page_to_number_version = {}

    for i, text in enumerate(texts):
        number = extract_number(text)       # your existing function
        version = version_detection(text)   # uses your working version_detection
        if number:
            if not version:
                version = "1"  # fallback
            page_to_number_version[i] = (number, version)

    grouped_pages = {}
    for page, num_ver in page_to_number_version.items():
        grouped_pages.setdefault(num_ver, []).append(page)

    return grouped_pages


def split_pdf(pdf_path):
	try:

		# Get the directory where the script is located
		script_dir = os.path.dirname(__file__)
		poppler_path = os.path.join(script_dir, "poppler", "Library", "bin")
		tesseract_path = os.path.join(script_dir, "tesseract-4.1.1", "tesseract.exe")

		pytesseract.pytesseract.tesseract_cmd = tesseract_path

		# Read the PDF
		reader = PdfReader(pdf_path)
		print(f"PDF has {len(reader.pages)} pages")

		# Convert first page to image
		print("Splitting PDF...")
		images = convert_from_path(pdf_path, poppler_path=poppler_path)

		output_dir = os.path.join(script_dir, "splits")
		os.makedirs(output_dir, exist_ok=True)
		
		texts = ocr_all_pages(images)

		h_pages = coa_detection(texts)

		p_pages = pilot_detection(texts)

		p_type = page_type(texts)

		if h_pages:
			for i in range(len(h_pages)):
			    start = h_pages[i]
			    end = h_pages[i+1] if i+1 < len(h_pages) else len(images)
			    new_pdf = PdfWriter()
			    for j in range(start, end):
			        if p_type[j] == "CoA":
			            new_pdf.add_page(reader.pages[j])

			    output_file = os.path.join(output_dir, f"CoA_{i+1}.pdf")
			    new_pdf.write(output_file)


		for (quote_number, pilot_version), pages in p_pages.items():
		    new_pdf = PdfWriter()
		    for p in pages:
		        new_pdf.add_page(reader.pages[p])

		    out_dir = fr"F:\NFI\RID\Formulation\R&D Pilots\Pilots\QB-{quote_number}"
		    os.makedirs(out_dir, exist_ok=True)

		    version = pilot_version
		    output_file = os.path.join(out_dir, f"PilotReport_V{version}.pdf")

		    if os.path.exists(output_file):
		        raise FileExistsError(f'File already exists: {output_file}')

		    new_pdf.write(output_file)

		return "Success"

	except Exception as e:
		return f'Error: {e}'

def get_pdfs():
	pdf_list = [os.path.join(scans_path, f) for f in os.listdir(scans_path) if os.path.isfile(os.path.join(scans_path, f)) and f.lower().endswith(".pdf")]

	return pdf_list


def process_files(pdfs):
	if not pdfs:
		print("No PDFs Found")

	try:
		with ProcessPoolExecutor(max_workers=4) as pool:
			results = pool.map(split_pdf, pdfs)
		
			for r in results:
				print(r)

	except Exception as e:
		print(f'Error: {e}')

if __name__ == "__main__":
	process_files(get_pdfs())

