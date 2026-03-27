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
    return draw_symbol_variant(size, compact=size <= 32)


def draw_symbol_variant(size: int, compact: bool) -> Image.Image:
    symbol = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    draw = ImageDraw.Draw(symbol)

    def p(x: float, y: float) -> tuple[int, int]:
        return (int(size * x), int(size * y))

    if compact:
        left_leg = [p(0.19, 0.84), p(0.38, 0.19), p(0.53, 0.19), p(0.34, 0.84)]
        right_leg = [p(0.47, 0.19), p(0.62, 0.19), p(0.81, 0.84), p(0.66, 0.84)]
        crossbar = [*p(0.35, 0.53), *p(0.65, 0.67)]
        crossbar_radius = max(2, int(size * 0.08))
    else:
        left_leg = [p(0.287, 0.768), p(0.426, 0.234), p(0.508, 0.234), p(0.369, 0.768)]
        right_leg = [p(0.492, 0.234), p(0.574, 0.234), p(0.713, 0.768), p(0.631, 0.768)]
        crossbar = [*p(0.418, 0.498), *p(0.592, 0.566)]
        crossbar_radius = max(4, int(size * 0.012))

    draw.polygon(left_leg, fill=SYMBOL_COLOR)
    draw.polygon(right_leg, fill=SYMBOL_COLOR)

    draw.rounded_rectangle(crossbar, radius=crossbar_radius, fill=SYMBOL_COLOR)

    if size >= 24:
        check_width = max(3, int(size * (0.085 if compact else 0.055)))
        check_points = (
            [p(0.56, 0.70), p(0.64, 0.79), p(0.82, 0.58)]
            if compact
            else [p(0.553, 0.639), p(0.607, 0.694), p(0.729, 0.551)]
        )
        draw.line(
            check_points,
            fill=CHECK_COLOR,
            width=check_width,
            joint="curve",
        )
    return symbol


def compose_icon(size: int = 1024) -> Image.Image:
    canvas = draw_background(size)
    canvas.alpha_composite(draw_symbol_variant(size, compact=size <= 32))
    return canvas


def save_ico(path: Path, sizes: Iterable[int]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    size_list = list(sizes)
    frames = [compose_icon(size) for size in size_list]
    if not frames:
        raise ValueError("icon sizes cannot be empty")
    frames[0].save(path, format="ICO", append_images=frames[1:])


def main() -> None:
    APP_ICON_PNG.parent.mkdir(parents=True, exist_ok=True)
    WINDOWS_ICON_ICO.parent.mkdir(parents=True, exist_ok=True)

    icon = compose_icon(1024)
    icon.save(APP_ICON_PNG, format="PNG")
    # The ICO file uses dedicated renderings per size.
    save_ico(WINDOWS_ICON_ICO, sizes=[256, 128, 64, 48, 32, 24, 16])

    print(f"Generated: {APP_ICON_PNG}")
    print(f"Generated: {WINDOWS_ICON_ICO}")


if __name__ == "__main__":
    main()
