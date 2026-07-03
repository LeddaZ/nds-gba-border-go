# nds-gba-border-go

Go tool to generate a `.bmp` GBA border image for NDS loaders/flashcarts that support them.

Supported formats:
- 8bpp ([GBARunner3](https://github.com/Gericom/GBARunner3), [GBA exploader](https://www.gamebrew.org/wiki/GBA_exploader))
- 15bpp ([AKMenu/AKAIO](https://github.com/lifehackerhansol/akmenu4), [AKMenu-Next](https://github.com/coderkei/akmenu-next))
- 24bpp ([YSMenu](https://www.gamebrew.org/wiki/YSMenu), [Boot GBA with Frame](https://www.gamebrew.org/wiki/Boot_GBA_with_Frame), [GBA exploader](https://www.gamebrew.org/wiki/GBA_exploader))

Tested on a DS Lite with AKMenu-Next 2.2.1 running from a DSpico.

## Usage
- The source image can be a `.bmp`, `.png`, `.jpg` or `.jpeg` file.
- The source image must be EXACTLY 256x192 pixels, any other size won't work.
- The output image will be saved as a Bitmap with the specified colour format.

Download the latest release from the [Releases](https://github.com/LeddaZ/nds-gba-border-go/releases) page for your OS and architecture. Builds are available for Windows, macOS and Linux x86 (excluding macOS), amd64 and arm64; run the executable from a terminal/Command Prompt/PowerShell window with the following parameters:

```
./nds-gba-border-go-{os}-{arch} <source_image> <output_image>
```

Where:
- `<source_image>` is the path to the source image file.
- `<output_image>` is the path to the output image file.

Example:
```
./nds-gba-border-go-linux-amd64 input.png output.bmp
```

Troubleshooting:
- Missing permissions on Linux/macOS: you may need to make the binary executable by running `chmod +x nds-gba-border-go-{os}-{arch}`
- The border isn't displayed correctly: make sure you chose the correct format, the file is in the correct location and the filename is correct. Check your loader's documentation for more information. If you are sure everything is correct, open an issue here and I'll look into it. **Keep in mind that I can't test YSMenu, as I don't have a flashcart that supports it.**
