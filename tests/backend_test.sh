#!/usr/bin/env bash
rm -rf out_mupdf out_poppler
mkdir out_mupdf
mkdir out_poppler

hyperfine "mudraw -r 72 -o out_mupdf/test_241_%d.ppm test_241.pdf"
hyperfine "pdftoppm -r 72 -progress test_241.pdf out_poppler/test_241"
