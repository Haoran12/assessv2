from __future__ import annotations

from pathlib import Path
from typing import Iterable

from PIL import Image, ImageDraw


ROOT = Path(__file__).resolve().parents[1]
BUILD_DIR = ROOT / "backend" / "desktop" / "build"
APP_ICON_PNG = BUILD_DIR / "appicon.png"
WINDOWS_ICON_ICO = BUILD_DIR / "windows" / "icon.ico"

BG_COLOR = (46, 111, 182, 255)
BORDER_COLOR = (29, 79, 137, 255)
SYMBOL_COLOR = (255, 255, 255, 255)
CHECK_COLOR = (52, 207, 161, 255)


def draw_background(size: int) -> Image.Image:
    radius = int(size * 0.21)

    rounded_mask = Image.new("L", (size, size), 0)
    mask_draw = ImageDraw.Draw(rounded_mask)
    margin = int(size * 0.072)
    mask_draw.rounded_rectangle(
        [margin, margin, size - margin, size - margin], radius=radius, fill=255
    )

    base = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    base.paste(Image.new("RGBA", (size, size), BG_COLOR), (0, 0), rounded_mask)

    border = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    border_draw = ImageDraw.Draw(border)
    border_draw.rounded_rectangle(
        [margin, margin, size - margin, size - margin],
        radius=radius,
        outline=BORDER_COLOR,
        width=max(2, int(size * 0.01)),
    )
    base.alpha_composite(border)
    return base


def draw_symbol(size: int) -> Image.Image:
    symbol = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    draw = ImageDraw.Draw(symbol)

    def p(x: float, y: float) -> tuple[int, int]:
        return (int(size * x), int(size * y))

    left_leg = [p(0.287, 0.768), p(0.426, 0.234), p(0.508, 0.234), p(0.369, 0.768)]
    right_leg = [p(0.492, 0.234), p(0.574, 0.234), p(0.713, 0.768), p(0.631, 0.768)]
    draw.polygon(left_leg, fill=SYMBOL_COLOR)
    draw.polygon(right_leg, fill=SYMBOL_COLOR)

    draw.rounded_rectangle(
        [*p(0.418, 0.498), *p(0.592, 0.498 + 0.068)],
        radius=max(4, int(size * 0.012)),
        fill=SYMBOL_COLOR,
    )

    check_width = max(4, int(size * 0.055))
    draw.line(
        [p(0.553, 0.639), p(0.607, 0.694), p(0.729, 0.551)],
        fill=CHECK_COLOR,
        width=check_width,
        joint="curve",
    )
    return symbol


def compose_icon(size: int = 1024) -> Image.Image:
    canvas = draw_background(size)
    canvas.alpha_composite(draw_symbol(size))
    return canvas


def save_ico(img: Image.Image, path: Path, sizes: Iterable[tuple[int, int]]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    img.save(path, format="ICO", sizes=list(sizes))


def main() -> None:
    APP_ICON_PNG.parent.mkdir(parents=True, exist_ok=True)
    WINDOWS_ICON_ICO.parent.mkdir(parents=True, exist_ok=True)

    icon = compose_icon(1024)
    icon.save(APP_ICON_PNG, format="PNG")
    save_ico(
        icon,
        WINDOWS_ICON_ICO,
        sizes=[(16, 16), (24, 24), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)],
    )

    print(f"Generated: {APP_ICON_PNG}")
    print(f"Generated: {WINDOWS_ICON_ICO}")


if __name__ == "__main__":
    main()
