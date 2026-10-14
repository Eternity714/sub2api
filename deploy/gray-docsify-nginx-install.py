#!/usr/bin/env python3
"""Prepare a single HTTPS server include and preserve its inherited add_header rules."""
import pathlib
import re
import sys


def statements(source: str):
    # Braces and semicolons inside strings/comments are not Nginx syntax.
    pattern = r'''\#.*?$|"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|[{};]|[^\s{};\#"']+'''
    tokens = [match for match in re.finditer(pattern, source, re.MULTILINE) if not match.group().startswith("#")]
    scopes = []
    current = []
    for token in tokens:
        text = token.group()
        if text == "{":
            scopes.append((current, token.start()))
            current = []
        elif text == "}":
            if not scopes:
                raise ValueError("Unbalanced Nginx braces")
            prefix, start = scopes.pop()
            yield "block", prefix, start, token.start(), len(scopes)
            current = []
        elif text == ";":
            yield "directive", current, current[0].start() if current else token.start(), token.end(), len(scopes)
            current = []
        else:
            current.append(token)
    if scopes:
        raise ValueError("Unbalanced Nginx braces")


def main():
    if len(sys.argv) != 6:
        raise ValueError("Usage: gray-docsify-nginx-install.py main patched-main snippet snippet-path server-name")
    main_path, patched_path, snippet_output, snippet_path, server_name = sys.argv[1:]
    if not re.fullmatch(r"/[A-Za-z0-9_./-]+", snippet_path):
        raise ValueError("Nginx include path contains unsupported characters")
    source = pathlib.Path(main_path).read_text()
    parsed = list(statements(source))
    servers = []
    for kind, prefix, start, end, depth in parsed:
        if kind != "block" or [token.group() for token in prefix] != ["server"] or depth != 0:
            continue
        direct = [(tokens, first, last) for entry_kind, tokens, first, last, entry_depth in parsed
                  if entry_kind == "directive" and entry_depth == 1 and start < first < end]
        names = [token.group().strip('"\'') for tokens, _, _ in direct if tokens and tokens[0].group() == "server_name" for token in tokens[1:]]
        listens = [[token.group() for token in tokens[1:]] for tokens, _, _ in direct if tokens and tokens[0].group() == "listen"]
        https = any("ssl" in listen and any(re.search(r"(?:^|:)443$", value) for value in listen) for listen in listens)
        if server_name in names and https:
            if any(tokens and tokens[0].group() == "return" for tokens, _, _ in direct):
                raise ValueError("Selected HTTPS server redirects all requests")
            servers.append((end, direct))
    if len(servers) != 1:
        raise ValueError("Expected exactly one HTTPS business server for " + server_name)
    end, direct = servers[0]
    includes = [[token.group().strip('"\'') for token in tokens[1:]] for tokens, _, _ in direct if tokens and tokens[0].group() == "include"]
    if sum(values == [snippet_path] for values in includes) > 1:
        raise ValueError("Documentation locations are included more than once")
    if [snippet_path] not in includes:
        source = source[:end] + f"    include {snippet_path};\n" + source[end:]
    headers = [source[first:last].strip() for tokens, first, last in direct if tokens and tokens[0].group() == "add_header"]
    # We add a cookie/release header, so Nginx would otherwise stop inheriting the
    # HTTPS server's HSTS/security/release headers. Copy their exact directives.
    common = [*headers, "add_header X-Docs-Release $sub2api_docsify_release always;",
              'add_header Set-Cookie "sub2api_docsify=$sub2api_docsify_cookie_value; Path=/docs; Secure; HttpOnly; SameSite=Lax" always;',
              "proxy_set_header Host $host;", "proxy_set_header X-Real-IP $remote_addr;",
              "proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;",
              "proxy_set_header X-Forwarded-Proto $scheme;", "proxy_connect_timeout 5s;", "proxy_read_timeout 30s;"]
    locations = [
        ("= /docs", ["proxy_pass $sub2api_docsify_entry_target$request_uri;"]),
        ("^~ /docs/_assets/", ["proxy_pass $sub2api_docsify_content_target$request_uri;"]),
        ("^~ /docs/assets/", ["proxy_pass $sub2api_docsify_legacy_assets_target$request_uri;"]),
        ("^~ /docs/", ["set $sub2api_docsify_proxy_target $sub2api_docsify_entry_target;",
                         "if ($uri ~* \\.(?:md|png|jpe?g|gif|ico|svg|webp)$) { set $sub2api_docsify_proxy_target $sub2api_docsify_content_target; }",
                         "proxy_pass $sub2api_docsify_proxy_target$request_uri;"]),
    ]
    snippet = "# Managed by gray Docsify scripts. Keep outside conf.d's automatic HTTP includes.\n"
    for location, directives in locations:
        snippet += f"location {location} {{\n" + "".join(f"    {line}\n" for line in [*common, *directives]) + "}\n"
    pathlib.Path(patched_path).write_text(source)
    pathlib.Path(snippet_output).write_text(snippet)


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError) as error:
        print("Cannot prepare Docsify Nginx include: " + str(error), file=sys.stderr)
        sys.exit(1)
