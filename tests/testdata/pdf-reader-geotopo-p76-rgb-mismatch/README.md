# GeoTopo p76 RGB mismatch fixture

Source PDF image: extracted from go-pdf test sample 009-pdflatex-geotopo/GeoTopo-komprimiert.pdf page 76, image 000.
Image size: 359x372 RGB, baseline JPEG, SOF0 components 1:2x2 2:1x1 3:1x1, no APP14 Adobe marker.

Files:
- input-geotopo-p76-rgb.jpg: original JPEG stream extracted by pdfimages -j.
- reference-poppler-pdfimages.png: Poppler pdfimages -png reference bitmap.
- reference-imagemagick-convert.png: ImageMagick/libjpeg conversion, exact 100 vs Poppler reference.
- actual-djpeg-go-default.ppm/png: current djpeg-go --ppm output.
- actual-djpeg-go-nosmooth.ppm/png: current djpeg-go --dct int --nosmooth --ppm output.
- actual-djpeg-go-turbo-fancy.ppm/png: djpeg-go --turbo-fancy output, matching Poppler/ImageMagick.

Measured with go-pdf cmd/splash_pixel_diff:
- reference-imagemagick-convert.png vs reference-poppler-pdfimages.png: 133548/133548 exact, 100%.
- actual-djpeg-go-default.png vs reference-poppler-pdfimages.png: 83613/133548 exact, 62.6089%; mismatched 49935.
- actual-djpeg-go-nosmooth.png vs reference-poppler-pdfimages.png: 81453/133548 exact, 60.9916%; mismatched 52095.
- actual-djpeg-go-turbo-fancy.png vs reference-poppler-pdfimages.png: 133548/133548 exact, 100%.

Repro commands from /workspace/djpeg-go:

```bash
go build -o /tmp/djpeg-go ./cmd/djpeg
/tmp/djpeg-go --ppm \
  --outfile /tmp/geotopo-p76-default.ppm \
  tests/testdata/pdf-reader-geotopo-p76-rgb-mismatch/input-geotopo-p76-rgb.jpg
/tmp/djpeg-go --dct int --nosmooth --ppm \
  --outfile /tmp/geotopo-p76-nosmooth.ppm \
  tests/testdata/pdf-reader-geotopo-p76-rgb-mismatch/input-geotopo-p76-rgb.jpg
/tmp/djpeg-go --turbo-fancy --ppm \
  --outfile /tmp/geotopo-p76-turbo-fancy.ppm \
  tests/testdata/pdf-reader-geotopo-p76-rgb-mismatch/input-geotopo-p76-rgb.jpg
convert /tmp/geotopo-p76-default.ppm /tmp/geotopo-p76-default.png
convert /tmp/geotopo-p76-nosmooth.ppm /tmp/geotopo-p76-nosmooth.png
convert /tmp/geotopo-p76-turbo-fancy.ppm /tmp/geotopo-p76-turbo-fancy.png
```

Compare the generated PNG files against `reference-poppler-pdfimages.png`.
