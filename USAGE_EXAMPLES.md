# M3U8 Downloader - Usage Examples

## Custom Headers

### Why use multiple `-header` flags?

The `-header` flag can be specified multiple times to add custom HTTP headers. This is especially important when header values contain commas, semicolons, or other special characters.

### Examples

#### Single header
```bash
m3u8-downloader.exe -url "https://example.com/playlist.m3u8" -header "User-Agent:Mozilla/5.0"
```

#### Multiple headers
```bash
m3u8-downloader.exe -url "https://example.com/playlist.m3u8" \
  -header "User-Agent:Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36" \
  -header "Referer:https://example.com" \
  -header "Origin:https://example.com"
```

#### Headers with commas (Cookie example)
```bash
# This works correctly because each header is separate
m3u8-downloader.exe -url "https://example.com/playlist.m3u8" \
  -header "Cookie:session_id=abc123, user_pref=dark_mode, lang=en_US" \
  -header "Accept:text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"
```

#### Common header combinations

**For sites requiring browser-like requests:**
```bash
m3u8-downloader.exe -url "https://example.com/playlist.m3u8" \
  -header "User-Agent:Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36" \
  -header "Accept:*/*" \
  -header "Accept-Language:en-US,en;q=0.9" \
  -header "Referer:https://example.com/videos/"
```

**For sites with authentication:**
```bash
m3u8-downloader.exe -url "https://example.com/playlist.m3u8" \
  -header "Authorization:Bearer your_token_here" \
  -header "User-Agent:Mozilla/5.0"
```

**For sites checking origin:**
```bash
m3u8-downloader.exe -url "https://cdn.example.com/playlist.m3u8" \
  -header "Origin:https://example.com" \
  -header "Referer:https://example.com/watch?v=12345"
```

## Custom Encryption Key

### When to use `-key`

Use the `-key` flag when:
- The encryption key URL in the M3U8 file is protected/requires authentication
- You've manually downloaded the encryption key
- The key server is blocking automated requests

### Example

```bash
# First, manually download the key (16 bytes for AES-128)
# Then use it with the downloader
m3u8-downloader.exe -url "https://example.com/playlist.m3u8" \
  -key "path/to/encryption.key" \
  -output "video.ts"
```

### Combined: Custom key + headers

```bash
m3u8-downloader.exe -url "https://example.com/protected_playlist.m3u8" \
  -key "encryption.key" \
  -header "User-Agent:Mozilla/5.0" \
  -header "Referer:https://example.com" \
  -header "Cookie:session=xyz123" \
  -output "video.mp4"
```

## Advanced Scenarios

### Downloading from a protected CDN
```bash
m3u8-downloader.exe -url "https://cdn.example.com/stream/playlist.m3u8" \
  -header "User-Agent:Mozilla/5.0 (Windows NT 10.0; Win64; x64)" \
  -header "Origin:https://www.example.com" \
  -header "Referer:https://www.example.com/watch" \
  -concurrent 5 \
  -timeout 60 \
  -retries 5 \
  -output "output.mp4"
```

### Local M3U8 with custom key and base URL
```bash
m3u8-downloader.exe -url "local_playlist.m3u8" \
  -baseurl "https://cdn.example.com/videos/" \
  -key "decryption.key" \
  -header "User-Agent:Custom Client 1.0" \
  -output "decrypted_video.ts"
```

## Tips

1. **Check browser DevTools**: Open the Network tab to see what headers the browser sends
2. **Copy User-Agent**: Use the exact User-Agent string from your browser
3. **Include Referer**: Many sites check if the request comes from their own domain
4. **Test headers first**: Try with just User-Agent, then add more headers if needed
5. **Watch for rate limiting**: Reduce `-concurrent` if you're getting errors
