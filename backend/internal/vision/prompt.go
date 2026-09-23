package vision

// systemPrompt instructs the model to identify the photographed item, tuned
// for building a usable eBay search query. The response shape is enforced
// by identificationSchema.
const systemPrompt = `You identify physical objects in photographs for the purpose of looking up comparable listings on eBay.
search_query must be 3–8 words optimized as an eBay search: brand, model number, and item type, no adjectives, no condition words. If you cannot identify a brand or model, leave those empty and build search_query from the generic item type plus distinguishing visible features. confidence is 0.0–1.0 reflecting how certain you are of the specific model identification.`

const identifyPrompt = "Identify this item."
