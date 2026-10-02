//! Repository automation: `cargo xtask <command>` (run from `core/`).

mod api;
mod i18n;
mod messages;
mod out;
mod package;
mod tokens;

use std::process::ExitCode;

const USAGE: &str = "usage: cargo xtask <command>

commands:
  codegen   regenerate every artifact derived from contract/ and the core's message types
  apple     build CouchverseCoreFFI.xcframework and the Swift bindings
  android   build the Android libraries, the JVM test library and the Kotlin bindings
  wasm      build the web's wasm package";

fn main() -> ExitCode {
    let args: Vec<String> = std::env::args().skip(1).collect();
    let result = match args.first().map(String::as_str) {
        Some("codegen") => codegen(),
        Some("apple") => package::apple(),
        Some("android") => package::android(),
        Some("wasm") => package::wasm(),
        Some(other) => Err(format!("unknown command {other}\n{USAGE}")),
        None => {
            eprintln!("{USAGE}");
            return ExitCode::from(2);
        }
    };
    match result {
        Ok(()) => ExitCode::SUCCESS,
        Err(err) => {
            eprintln!("error: {err}");
            ExitCode::FAILURE
        }
    }
}

fn codegen() -> Result<(), String> {
    let root = out::repo_root();
    let mut written = Vec::new();
    i18n::generate(&root, &mut written)?;
    tokens::generate(&root, &mut written)?;
    api::generate(&root, &mut written)?;
    messages::generate(&root, &mut written)?;
    for path in &written {
        println!("wrote {}", path.strip_prefix(&root).unwrap_or(path).display());
    }
    Ok(())
}
