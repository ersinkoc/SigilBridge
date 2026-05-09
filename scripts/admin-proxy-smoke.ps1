$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.Net.Http

function Invoke-AdminProxyWriteSmoke {
  param(
    [int]$AdminPort,
    [string]$CookieHeader
  )

  $uri = "http://127.0.0.1:$AdminPort/admin/v1/pools/mock-chat/probe"
  $publicOrigin = "https://bridge.example.test"
  $publicHost = "bridge.example.test"

  $badResponse = Invoke-AdminProxyWriteRequest -Uri $uri -PublicOrigin $publicOrigin -PublicHost $publicHost -CookieHeader $CookieHeader
  if ($badResponse.StatusCode -ne 403) {
    throw "Expected proxied admin write without X-Forwarded-Proto to return 403, got $($badResponse.StatusCode): $($badResponse.Body)"
  }

  $goodResponse = Invoke-AdminProxyWriteRequest -Uri $uri -PublicOrigin $publicOrigin -PublicHost $publicHost -CookieHeader $CookieHeader -ForwardedProto "https"
  if ($goodResponse.StatusCode -ne 200) {
    throw "Expected proxied admin write with X-Forwarded-Proto to return 200, got $($goodResponse.StatusCode): $($goodResponse.Body)"
  }
  $probe = $goodResponse.Body | ConvertFrom-Json
  if (-not $probe.ok) {
    throw "Proxied admin write smoke did not return ok=true: $($probe | ConvertTo-Json -Compress)"
  }
  return $probe
}

function Invoke-AdminProxyWriteRequest {
  param(
    [string]$Uri,
    [string]$PublicOrigin,
    [string]$PublicHost,
    [string]$CookieHeader,
    [string]$ForwardedProto
  )

  $handler = [System.Net.Http.HttpClientHandler]::new()
  $handler.UseCookies = $false
  $client = [System.Net.Http.HttpClient]::new($handler)
  try {
    $request = [System.Net.Http.HttpRequestMessage]::new([System.Net.Http.HttpMethod]::Post, $Uri)
    $request.Headers.Host = $PublicHost
    $request.Headers.TryAddWithoutValidation("Origin", $PublicOrigin) | Out-Null
    $request.Headers.TryAddWithoutValidation("Cookie", $CookieHeader) | Out-Null
    if ($ForwardedProto) {
      $request.Headers.TryAddWithoutValidation("X-Forwarded-Proto", $ForwardedProto) | Out-Null
    }
    $request.Content = [System.Net.Http.StringContent]::new("{}", [System.Text.Encoding]::UTF8, "application/json")
    $response = $client.SendAsync($request).GetAwaiter().GetResult()
    return [pscustomobject]@{
      StatusCode = [int]$response.StatusCode
      Body = $response.Content.ReadAsStringAsync().GetAwaiter().GetResult()
    }
  } finally {
    $client.Dispose()
    $handler.Dispose()
  }
}
