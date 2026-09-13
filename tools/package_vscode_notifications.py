#!/usr/bin/env python3
"""Build the optional local VS Code extension without network/npm dependencies."""
import argparse
import json
from pathlib import Path
import xml.etree.ElementTree as ET
import zipfile

SOURCE = Path(__file__).resolve().parents[1] / "clients/vscode-notify"


def package(output):
    metadata = json.loads((SOURCE / "package.json").read_text(encoding="utf-8"))
    namespace = "http://schemas.microsoft.com/developer/vsx-schema/2011"
    ET.register_namespace("", namespace)
    def tag(name):
        return "{" + namespace + "}" + name
    root = ET.Element(tag("PackageManifest"), Version="2.0.0")
    details = ET.SubElement(root, tag("Metadata"))
    ET.SubElement(details, tag("Identity"), Language="en-US", Id=metadata["name"],
                  Version=metadata["version"], Publisher=metadata["publisher"])
    ET.SubElement(details, tag("DisplayName")).text = metadata["displayName"]
    ET.SubElement(details, tag("Description")).text = metadata["description"]
    properties = ET.SubElement(details, tag("Properties"))
    ET.SubElement(properties, tag("Property"), Id="Microsoft.VisualStudio.Code.Engine", Value=metadata["engines"]["vscode"])
    installation = ET.SubElement(root, tag("Installation"))
    ET.SubElement(installation, tag("InstallationTarget"), Id="Microsoft.VisualStudio.Code")
    ET.SubElement(root, tag("Dependencies"))
    assets = ET.SubElement(root, tag("Assets"))
    ET.SubElement(assets, tag("Asset"), Type="Microsoft.VisualStudio.Code.Manifest", Path="extension/package.json", Addressable="true")
    content_types = '<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="json" ContentType="application/json"/><Default Extension="js" ContentType="application/javascript"/><Default Extension="png" ContentType="image/png"/><Default Extension="md" ContentType="text/markdown"/><Default Extension="vsixmanifest" ContentType="text/xml"/></Types>'
    with zipfile.ZipFile(output, "x", zipfile.ZIP_DEFLATED) as archive:
        archive.writestr("extension.vsixmanifest", ET.tostring(root, encoding="utf-8", xml_declaration=True))
        archive.writestr("[Content_Types].xml", content_types)
        for name in ("package.json", "extension.js", "review.js", "review_panel.js", "media/icon.png", "README.md"):
            if name == "README.md":
                # Repository-relative links do not exist in an installed VSIX.
                readme = (SOURCE / name).read_text(encoding="utf-8").replace(
                    "(../../docs/", "(https://github.com/SLktEx/Hacocoon/blob/main/docs/")
                archive.writestr("extension/" + name, readme)
            else:
                archive.write(SOURCE / name, "extension/" + name)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("output", type=Path, help="new .vsix output path (never overwritten)")
    package(parser.parse_args().output)
