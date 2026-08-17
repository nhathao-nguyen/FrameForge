export type OpaqueId = string & { readonly __opaqueId: unique symbol };

export type Revision = number & { readonly __revision: unique symbol };

export interface ApiErrorEnvelope {
  error: {
    code: string;
    message: string;
    request_id: OpaqueId;
    correlation_id: OpaqueId;
  };
  serialization_version: "1";
}
