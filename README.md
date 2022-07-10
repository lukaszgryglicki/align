# align

Align 3 images to create RGB image.

# Compiling

- `make` - you need to have go installed.

# Installing

- `make install`

# Results

You can create example aligned image via: `./align R.JPG G.JPG B.JPG Aligned.JPG`.

You can do the same using ImageMagick's `convert`: `convert R.JPG G.JPG B.JPG -combine Unaligned.JPG`.

But `convert` will not try to aliggn images together, it will only combine channels.

# Some details

Supported image types are: PNG, JPG, GIF, TIF, BMP.

Other example: `N=16 FROM_X=3000 FROM_Y=2000 RANGE_X=20 RANGE_y=20 SIZE_X=80 SIZE_Y=80 ./align R.JPG G.JPG B.JPG X.bmp`.

Defaults: it starts from smalles image center (they're supposed to be the same size) and checks squares 200 pixels size from center (so single check is 200+200+1 = 401x401 = 160801 pixels).

Then it performs movements 64 pixels in all directions (this gives 64+64+1 = 129x129 = 16641/16.6K checks each checking 161K pixels).

You can also override number of (v)CPUs autodetection and specify for example N=8.

You can change those details via environmental variables.

# Environment variables:

- `N` - how many (v)CPUs use, defaults to autodetect.
- `FROM_X` - where is the x value for image circle to start aligning from (middle of the first image if not specified).
- `FROM_Y` - where is the y value for image circle to start aligning from (middle of the first image if not specified).
- `RANGE_X` - how many x pixels check around start x (defaults to 64, which gives 64+64+1 = 129 checks, 129x129 = 16641 checks for X & Y - if set the same).
- `RANGE_Y` - how many y pixels check around start y (defaults to 64, which gives 64+64+1 = 129 checks, 129x129 = 16641 checks for X & Y - if set the same).
- `SIZE_X` - how many x pixels check in single pass (defaults to 200, which gives 200+200+1 = 401x401 = 160801 pixels - if x & y pixels set the same).
- `SIZE_Y` - how many y pixels check in single pass (defaults to 200, which gives 200+200+1 = 401x401 = 160801 pixels - if x & y pixels set the same).

