"""Generate Android and web vector resources from assets/icon.svg (stdlib only)."""

from pathlib import Path
import argparse
import xml.etree.ElementTree as ET

ROOT = Path(__file__).resolve().parents[1]
ANDROID = ROOT / "android/app/src/main/res"
NS = "http://schemas.android.com/apk/res/android"
ET.register_namespace("android", NS)


def vector(paths, *, adaptive=False, size=108):
    attrs = {"width": f"{size}dp", "height": f"{size}dp",
             "viewportWidth": "108", "viewportHeight": "108"}
    result = ET.Element("vector", {f"{{{NS}}}{k}": v for k, v in attrs.items()})
    parent = result
    if adaptive:
        # Keep the whole phone/monitor inside circular adaptive-icon masks.
        parent = ET.SubElement(result, "group", {
            f"{{{NS}}}{k}": v for k, v in
            {"pivotX": "54", "pivotY": "54", "scaleX": "0.70", "scaleY": "0.70"}.items()
        })
    for path in paths:
        attrs = {f"{{{NS}}}fillColor": path.attrib["fill"],
                 f"{{{NS}}}pathData": path.attrib["d"]}
        if path.get("fill-rule") == "evenodd":
            attrs[f"{{{NS}}}fillType"] = "evenOdd"
        ET.SubElement(parent, "path", attrs)
    ET.indent(result, space="    ")
    return ET.tostring(result, encoding="unicode") + "\n"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true", help="Fail on stale resources without writing")
    check = parser.parse_args().check
    source = ROOT / "assets/icon.svg"
    paths = list(ET.parse(source).getroot())
    assert paths[0].get("id") == "background"
    foreground = paths[1:]
    outputs = {
        "app_icon.xml": vector(paths),
        "icon_foreground.xml": vector(foreground, adaptive=True),
        "icon_monochrome.xml": vector(foreground, adaptive=True),
        "ic_notification.xml": vector(foreground, size=24),
    }
    files = {ANDROID / "drawable" / name: text for name, text in outputs.items()}
    files[ROOT / "deploy/icon.svg"] = source.read_text(encoding="utf-8")
    files[ROOT / "web/icon.svg"] = source.read_text(encoding="utf-8")
    for path, text in files.items():
        if check:
            assert path.read_text(encoding="utf-8") == text, f"Stale icon: {path}"
        else:
            path.write_text(text, encoding="utf-8")
    print(f"{'Checked' if check else 'Generated'} four Android vectors and three web icons")


if __name__ == "__main__":
    main()
