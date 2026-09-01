using System.Text.Encodings.Web;
using System.Text.Json;
using System.Text.Unicode;
using Microsoft.AspNetCore.Http;

namespace Caro.Api;

/// <summary>
/// Shared serializer options: every serialized name is explicit via
/// [JsonPropertyName], so the naming policy stays off. The encoder must
/// leave every code point alone so error messages round-trip like Go's
/// encoding/json.
/// </summary>
public static class JsonOptions
{
    public static readonly JsonSerializerOptions Shared = new(JsonSerializerDefaults.Web)
    {
        PropertyNamingPolicy = null,
        Encoder = JavaScriptEncoder.Create(UnicodeRanges.All),
    };
}

public static class ResponseJson
{
    public static async Task Write(HttpContext http, int status, object value)
    {
        http.Response.StatusCode = status;
        http.Response.ContentType = "application/json";
        await http.Response.WriteAsync(JsonSerializer.Serialize(value, JsonOptions.Shared));
    }
}
