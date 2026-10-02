//! Design tokens: `contract/design/tokens.json` -> a CSS `@theme` include and a TypeScript module
//! (web), `DesignTokens.swift` (Apple) and `Tokens.kt` (Android).

use std::f64::consts::PI;
use std::fmt::Write as _;
use std::path::{Path, PathBuf};

use serde_json::{Map, Value};

use crate::out;

const SOURCE: &str = "contract/design/tokens.json";
const WEB_DIR: &str = "clients/web/src/lib/generated";
const APPLE_FILE: &str =
    "clients/apple/Packages/CouchverseDesign/Sources/CouchverseDesign/Generated/DesignTokens.swift";
const ANDROID_FILE: &str =
    "clients/android/design/src/generated/kotlin/io/stepes/couchverse/design/Tokens.kt";

pub fn generate(root: &Path, written: &mut Vec<PathBuf>) -> Result<(), String> {
    let tokens = Tokens::read(&root.join(SOURCE))?;
    out::write(&root.join(WEB_DIR).join("tokens.css"), &css(&tokens)?, written)?;
    out::write(&root.join(WEB_DIR).join("tokens.ts"), &typescript(&tokens)?, written)?;
    out::write(&root.join(APPLE_FILE), &swift(&tokens)?, written)?;
    out::write(&root.join(ANDROID_FILE), &kotlin(&tokens)?, written)?;
    Ok(())
}

struct Tokens(Value);

impl Tokens {
    fn read(path: &Path) -> Result<Self, String> {
        Ok(Self(out::read_json(path)?))
    }

    /// The entries of a token group, without `$comment`-style metadata.
    fn group(&self, path: &[&str]) -> Result<Vec<(&String, &Value)>, String> {
        let mut value = &self.0;
        for key in path {
            value =
                value.get(key).ok_or_else(|| format!("{SOURCE}: missing {}", path.join(".")))?;
        }
        let object: &Map<String, Value> = value
            .as_object()
            .ok_or_else(|| format!("{SOURCE}: {} is not an object", path.join(".")))?;
        Ok(object.iter().filter(|(k, _)| !k.starts_with('$')).collect())
    }

    fn number(&self, path: &[&str]) -> Result<f64, String> {
        let (last, parent) = path.split_last().expect("non-empty path");
        self.group(parent)?
            .into_iter()
            .find(|(k, _)| k.as_str() == *last)
            .and_then(|(_, v)| v.as_f64())
            .ok_or_else(|| format!("{SOURCE}: {} must be a number", path.join(".")))
    }
}

fn num(value: &Value, what: &str) -> Result<f64, String> {
    value.as_f64().ok_or_else(|| format!("{SOURCE}: {what} must be a number"))
}

fn int(value: &Value, what: &str) -> Result<u64, String> {
    value.as_u64().ok_or_else(|| format!("{SOURCE}: {what} must be a whole number"))
}

fn text<'a>(value: &'a Value, what: &str) -> Result<&'a str, String> {
    value.as_str().ok_or_else(|| format!("{SOURCE}: {what} must be a string"))
}

/// `#rrggbb` or `#rrggbbaa` -> (rgb, alpha byte).
fn rgba(value: &Value, what: &str) -> Result<(u32, u8), String> {
    let hex = text(value, what)?;
    let digits = hex.strip_prefix('#').filter(|d| matches!(d.len(), 6 | 8));
    let parsed = digits.and_then(|d| u32::from_str_radix(d, 16).ok());
    match (digits, parsed) {
        (Some(d), Some(v)) if d.len() == 6 => Ok((v, 0xff)),
        (Some(_), Some(v)) => Ok((v >> 8, (v & 0xff) as u8)),
        _ => Err(format!("{SOURCE}: {what} must be #rrggbb or #rrggbbaa, got {hex}")),
    }
}

/// Short float literal for generated code: `0.86`, `-0.02`, `14`.
fn float(v: f64) -> String {
    let s = format!("{v:.4}");
    let s = s.trim_end_matches('0').trim_end_matches('.');
    if s == "-0" { "0".into() } else { s.into() }
}

fn css(t: &Tokens) -> Result<String, String> {
    let mut s = out::header("/*", SOURCE).replace(" Do not edit.\n", " Do not edit. */\n");
    s.push_str("@theme {\n");
    for (name, value) in t.group(&["color"])? {
        let _ = writeln!(s, "\t--color-{name}: {};", text(value, name)?);
    }
    for (name, value) in t.group(&["radius"])? {
        let _ = writeln!(s, "\t--radius-{name}: {}px;", float(num(value, name)?));
    }
    s.push_str("}\n");
    Ok(s)
}

fn typescript(t: &Tokens) -> Result<String, String> {
    let mut s = out::header("//", SOURCE);
    let record = |s: &mut String, name: &str, group: &[(&String, &Value)]| -> Result<(), String> {
        let _ = writeln!(s, "\nexport const {name} = {{");
        for (key, entry) in group {
            let fields: Vec<String> = entry
                .as_object()
                .ok_or_else(|| format!("{SOURCE}: {key} must be an object"))?
                .iter()
                .map(|(field, v)| format!("{field}: '{}'", v.as_str().unwrap_or_default()))
                .collect();
            let _ = writeln!(s, "\t{key}: {{ {} }},", fields.join(", "));
        }
        s.push_str("} as const;\n");
        Ok(())
    };
    record(&mut s, "tiers", &t.group(&["tier"])?)?;
    record(&mut s, "medals", &t.group(&["medal"])?)?;

    s.push_str("\nexport const accent = {\n");
    for (key, value) in t.group(&["accent"])? {
        match value {
            Value::String(v) => {
                let _ = writeln!(s, "\t{key}: '{v}',");
            }
            v => {
                let _ = writeln!(s, "\t{key}: {},", float(num(v, key)?));
            }
        }
    }
    s.push_str("} as const;\n");
    let _ = writeln!(
        s,
        "\nexport const heroIntervalMs = {};",
        float(t.number(&["motion", "heroInterval"])?)
    );
    Ok(s)
}

fn swift_color(value: &Value, what: &str) -> Result<String, String> {
    let (rgb, alpha) = rgba(value, what)?;
    if alpha == 0xff {
        Ok(format!("Color(hex: 0x{rgb:06X})"))
    } else {
        Ok(format!("Color(hex: 0x{rgb:06X}, opacity: {})", float(f64::from(alpha) / 255.0)))
    }
}

fn swift_weight(weight: u64) -> &'static str {
    match weight {
        ..=150 => "ultraLight",
        151..=250 => "thin",
        251..=350 => "light",
        351..=450 => "regular",
        451..=550 => "medium",
        551..=650 => "semibold",
        651..=750 => "bold",
        751..=850 => "heavy",
        _ => "black",
    }
}

const SWIFT_TYPES: &str = r"
    public struct TypeRole: Sendable {
        public let textStyle: Font.TextStyle
        public let weight: Font.Weight
        /// Letter spacing as a fraction of the font size.
        public let trackingEm: CGFloat
    }

    public struct TierPalette: Sendable {
        public let color: Color
        public let glow: Color
    }

    public struct MedalPalette: Sendable {
        public let from: Color
        public let to: Color
        public let ring: Color
        public let glow: Color
    }
";

const SWIFT_COLOR_INIT: &str = r"
extension Color {
    fileprivate init(hex: UInt32, opacity: Double = 1) {
        self.init(
            .sRGB,
            red: Double((hex >> 16) & 0xFF) / 255,
            green: Double((hex >> 8) & 0xFF) / 255,
            blue: Double(hex & 0xFF) / 255,
            opacity: opacity
        )
    }
}
";

fn swift(t: &Tokens) -> Result<String, String> {
    let mut s = out::header("//", SOURCE);
    s.push_str("\nimport SwiftUI\n\npublic enum Tokens {");
    s.push_str(SWIFT_TYPES);
    s.push_str(&swift_values(t)?);
    s.push_str(&swift_motion(t)?);
    s.push_str(&swift_palettes(t)?);
    s.push_str("}\n");
    s.push_str(SWIFT_COLOR_INIT);
    Ok(s)
}

fn swift_values(t: &Tokens) -> Result<String, String> {
    let mut s = String::from("\n    public enum Palette {\n");
    for (name, value) in t.group(&["color"])? {
        let color = swift_color(value, name)?;
        let _ = writeln!(s, "        public static let {} = {color}", out::camel(name));
    }
    s.push_str("    }\n\n    public enum Accent {\n");
    for (name, value) in t.group(&["accent"])? {
        let line = match value {
            Value::String(_) => format!("{name} = {}", swift_color(value, name)?),
            v => format!("{name}: Double = {}", float(num(v, name)?)),
        };
        let _ = writeln!(s, "        public static let {line}");
    }
    s.push_str("    }\n");
    for (group, ty) in [("radius", "Radius"), ("spacing", "Spacing")] {
        let _ = writeln!(s, "\n    public enum {ty} {{");
        for (name, value) in t.group(&[group])? {
            let v = float(num(value, name)?);
            let _ = writeln!(s, "        public static let {name}: CGFloat = {v}");
        }
        s.push_str("    }\n");
    }
    s.push_str("\n    public enum TypeRamp {\n");
    for (name, role) in t.group(&["type"])? {
        let _ = writeln!(
            s,
            "        public static let {name} = TypeRole(textStyle: .{}, weight: .{}, trackingEm: {})",
            text(&role["apple"], name)?,
            swift_weight(int(&role["weight"], name)?),
            float(num(&role["tracking"], name)?)
        );
    }
    s.push_str("    }\n");
    Ok(s)
}

fn swift_motion(t: &Tokens) -> Result<String, String> {
    let mut s = String::from("\n    public enum Motion {\n");
    for (name, spring) in t.group(&["motion", "spring"])? {
        let _ = writeln!(
            s,
            "        public static let {name} = Animation.spring(response: {}, dampingFraction: {})",
            float(num(&spring["response"], name)?),
            float(num(&spring["damping"], name)?)
        );
    }
    for (name, curve) in t.group(&["motion", "curve"])? {
        let points: Vec<String> = bezier(curve, name)?.into_iter().map(float).collect();
        let _ = writeln!(
            s,
            "        public static let {name} = Animation.timingCurve({}, duration: {})",
            points.join(", "),
            float(num(&curve["duration"], name)? / 1000.0)
        );
    }
    let interval = int(&t.0["motion"]["heroInterval"], "motion.heroInterval")?;
    let _ =
        writeln!(s, "        public static let heroInterval: Duration = .milliseconds({interval})");
    s.push_str("    }\n");
    Ok(s)
}

fn swift_palettes(t: &Tokens) -> Result<String, String> {
    let mut s = String::from("\n    public enum Tier {\n");
    let tiers = t.group(&["tier"])?;
    for (name, tier) in &tiers {
        let _ = writeln!(
            s,
            "        public static let {name} = TierPalette(color: {}, glow: {})",
            swift_color(&tier["color"], name)?,
            swift_color(&tier["glow"], name)?
        );
    }
    let first = tiers.first().map(|(n, _)| n.as_str()).ok_or("no tiers")?;
    s.push_str("\n        /// The palette for a tier code from the API; unknown codes fall back to the first.\n");
    s.push_str("        public static func named(_ code: String) -> TierPalette {\n");
    s.push_str("            switch code {\n");
    for (name, _) in &tiers {
        let _ = writeln!(s, "            case \"{name}\": {name}");
    }
    let _ = writeln!(s, "            default: {first}\n            }}\n        }}\n    }}");

    s.push_str("\n    public enum Medal {\n");
    for (name, medal) in t.group(&["medal"])? {
        let _ = writeln!(
            s,
            "        public static let {name} = MedalPalette(from: {}, to: {}, ring: {}, glow: {})",
            swift_color(&medal["from"], name)?,
            swift_color(&medal["to"], name)?,
            swift_color(&medal["ring"], name)?,
            swift_color(&medal["glow"], name)?
        );
    }
    s.push_str("    }\n");
    Ok(s)
}

fn bezier(curve: &Value, name: &str) -> Result<Vec<f64>, String> {
    curve["bezier"]
        .as_array()
        .filter(|p| p.len() == 4)
        .ok_or_else(|| format!("{SOURCE}: {name}.bezier needs 4 numbers"))?
        .iter()
        .map(|v| num(v, name))
        .collect()
}

fn kotlin_color(value: &Value, what: &str) -> Result<String, String> {
    let (rgb, alpha) = rgba(value, what)?;
    Ok(format!("Color(0x{alpha:02X}{rgb:06X})"))
}

fn kotlin_float(v: f64) -> String {
    format!("{}f", float(v))
}

/// `displaySmall` -> `DisplaySmall`.
fn pascal(name: &str) -> String {
    let mut chars = name.chars();
    chars.next().map(|c| c.to_uppercase().chain(chars).collect()).unwrap_or_default()
}

const KOTLIN_TYPES: &str = r"
    /** [trackingEm] is letter spacing as a fraction of the font size. */
    data class TypeRole(val material: MaterialRole, val weight: Int, val trackingEm: Float)

    data class SpringToken(val dampingRatio: Float, val stiffness: Float)

    data class CurveToken(val durationMillis: Int, val x1: Float, val y1: Float, val x2: Float, val y2: Float)

    data class TierPalette(val color: Color, val glow: Color)

    data class MedalPalette(val from: Color, val to: Color, val ring: Color, val glow: Color)
";

fn kotlin(t: &Tokens) -> Result<String, String> {
    let mut s = out::header("//", SOURCE);
    s.push_str("\npackage io.stepes.couchverse.design\n\n");
    s.push_str("import androidx.compose.ui.graphics.Color\nimport androidx.compose.ui.unit.dp\n\n");
    s.push_str("object Tokens {");
    s.push_str(KOTLIN_TYPES);
    s.push_str(&kotlin_values(t)?);
    s.push_str(&kotlin_motion(t)?);
    s.push_str(&kotlin_palettes(t)?);
    s.push_str("}\n");
    Ok(s)
}

fn kotlin_values(t: &Tokens) -> Result<String, String> {
    let mut s = String::from("\n    object Palette {\n");
    for (name, value) in t.group(&["color"])? {
        let _ = writeln!(s, "        val {} = {}", out::camel(name), kotlin_color(value, name)?);
    }
    s.push_str("    }\n\n    object Accent {\n");
    for (name, value) in t.group(&["accent"])? {
        let line = match value {
            Value::String(_) => format!("val {name} = {}", kotlin_color(value, name)?),
            v => format!("const val {name} = {}", kotlin_float(num(v, name)?)),
        };
        let _ = writeln!(s, "        {line}");
    }
    s.push_str("    }\n");
    for (group, ty) in [("radius", "Radius"), ("spacing", "Spacing")] {
        let _ = writeln!(s, "\n    object {ty} {{");
        for (name, value) in t.group(&[group])? {
            let _ = writeln!(s, "        val {name} = {}.dp", float(num(value, name)?));
        }
        s.push_str("    }\n");
    }

    let roles = t.group(&["type"])?;
    let mut materials: Vec<String> = Vec::new();
    for (name, role) in &roles {
        let material = pascal(text(&role["material"], name)?);
        if !materials.contains(&material) {
            materials.push(material);
        }
    }
    s.push_str("\n    /** Material 3 type scale entries; the phone and TV themes map them to their typography. */\n");
    let _ = writeln!(s, "    enum class MaterialRole {{ {} }}", materials.join(", "));
    s.push_str("\n    object TypeRamp {\n");
    for (name, role) in &roles {
        let _ = writeln!(
            s,
            "        val {name} = TypeRole(MaterialRole.{}, {}, {})",
            pascal(text(&role["material"], name)?),
            int(&role["weight"], name)?,
            kotlin_float(num(&role["tracking"], name)?)
        );
    }
    s.push_str("    }\n");
    Ok(s)
}

fn kotlin_motion(t: &Tokens) -> Result<String, String> {
    let mut s = String::from("\n    object Motion {\n");
    for (name, spring) in t.group(&["motion", "spring"])? {
        // SwiftUI's response is the undamped period; Compose wants stiffness = (2 pi / response)^2
        let stiffness = (2.0 * PI / num(&spring["response"], name)?).powi(2);
        let _ = writeln!(
            s,
            "        val {name} = SpringToken(dampingRatio = {}, stiffness = {})",
            kotlin_float(num(&spring["damping"], name)?),
            kotlin_float(stiffness.round())
        );
    }
    for (name, curve) in t.group(&["motion", "curve"])? {
        let points: Vec<String> = bezier(curve, name)?.into_iter().map(kotlin_float).collect();
        let _ = writeln!(
            s,
            "        val {name} = CurveToken({}, {})",
            int(&curve["duration"], name)?,
            points.join(", ")
        );
    }
    let interval = int(&t.0["motion"]["heroInterval"], "motion.heroInterval")?;
    let _ = writeln!(s, "        const val heroIntervalMillis = {interval}L");
    s.push_str("    }\n");
    Ok(s)
}

fn kotlin_palettes(t: &Tokens) -> Result<String, String> {
    let mut s = String::from("\n    object Tier {\n");
    let tiers = t.group(&["tier"])?;
    for (name, tier) in &tiers {
        let _ = writeln!(
            s,
            "        val {name} = TierPalette({}, {})",
            kotlin_color(&tier["color"], name)?,
            kotlin_color(&tier["glow"], name)?
        );
    }
    let first = tiers.first().map(|(n, _)| n.as_str()).ok_or("no tiers")?;
    s.push_str("\n        /** The palette for a tier code from the API; unknown codes fall back to the first. */\n");
    s.push_str("        fun named(code: String): TierPalette =\n            when (code) {\n");
    for (name, _) in &tiers {
        let _ = writeln!(s, "                \"{name}\" -> {name}");
    }
    let _ = writeln!(s, "                else -> {first}\n            }}\n    }}");

    s.push_str("\n    object Medal {\n");
    for (name, medal) in t.group(&["medal"])? {
        let _ = writeln!(
            s,
            "        val {name} = MedalPalette({}, {}, {}, {})",
            kotlin_color(&medal["from"], name)?,
            kotlin_color(&medal["to"], name)?,
            kotlin_color(&medal["ring"], name)?,
            kotlin_color(&medal["glow"], name)?
        );
    }
    s.push_str("    }\n");
    Ok(s)
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn colours_parse_with_and_without_alpha() {
        assert_eq!(rgba(&json!("#07080d"), "c").unwrap(), (0x0007_080d, 0xff));
        assert_eq!(rgba(&json!("#7c849659"), "c").unwrap(), (0x007c_8496, 0x59));
        assert!(rgba(&json!("#fff"), "c").is_err());
    }

    #[test]
    fn floats_print_short() {
        assert_eq!(float(14.0), "14");
        assert_eq!(float(-0.02), "-0.02");
        assert_eq!(float(0.86), "0.86");
        assert_eq!(float(-0.0), "0");
    }
}
