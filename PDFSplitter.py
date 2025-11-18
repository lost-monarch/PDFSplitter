from PyPDF2 import PdfReader, PdfWriter
from pdf2image import convert_from_path
import os
import re
import pytesseract

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


def img_detection(texts):
    header_phrases = ["Certificate of Analysis", "Specification Sheet"]
    return [
        i for i, text in enumerate(texts)
        if any(h in text for h in header_phrases)
    ]


def pilot_detection(texts):
    page_to_number = {}

    for i, text in enumerate(texts):
        number = extract_number(text)
        if number:
            page_to_number[i] = number

    quotation_to_pages = {}
    for page, number in page_to_number.items():
        quotation_to_pages.setdefault(number, []).append(page)

    return quotation_to_pages


def main():
	try:
		pdf_path = input("Enter the full path to your test PDF: ").strip('"')


		# Get the directory where the script is located
		script_dir = os.path.dirname(__file__)
		poppler_path = os.path.join(script_dir, "poppler", "Library", "bin")
		tesseract_path = os.path.join(script_dir, "tesseract-4.1.1", "tesseract.exe")

		pytesseract.pytesseract.tesseract_cmd = tesseract_path

		# Read the PDF
		reader = PdfReader(pdf_path)
		print(f"PDF has {len(reader.pages)} pages")

		# Convert first page to image
		print("Converting first page to image...")
		images = convert_from_path(pdf_path, poppler_path=poppler_path)

		output_dir = os.path.join(script_dir, "splits")
		os.makedirs(output_dir, exist_ok=True)
		
		texts = ocr_all_pages(images)

		h_pages = img_detection(texts)

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


		for number, pages in p_pages.items():
		    new_pdf = PdfWriter()
		    for p in pages:
		        new_pdf.add_page(reader.pages[p])
		    
		    output_file = os.path.join(output_dir, f"PilotReport_{number}.pdf")
		    new_pdf.write(output_file)



		# for i in images:
		# 	if p_type[i] == "CoA":
		# 		for i in range(len(h_pages)):
		# 			new_pdf = PdfWriter()
		# 			start = h_pages[i]
		# 			end = h_pages[i+1]
					
		# 			for j in range(start, end):
		# 				new_pdf.add_page(reader.pages[j])

		# 			output_file = os.path.join(output_dir, f"split_{i+1}.pdf")
		# 			new_pdf.write(output_file)

		# 	if p_type[i]
		
		# for number, pages in p_pages.items():
		#     new_pdf = PdfWriter()
		#     for p in pages:
		#         new_pdf.add_page(reader.pages[p])

		#     output_file = os.path.join(output_dir, f"PilotReport_{number}.pdf")
		#     new_pdf.write(output_file)


		print("PDF Split Successfully")

	except Exception as e:
		print(f'Error: {e}')

if __name__ == "__main__":
    main()