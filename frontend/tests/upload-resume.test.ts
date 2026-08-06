import { describe, expect, it } from "bun:test";

import { findUploadResume, listUploadResumes, removeUploadResume, saveUploadResume, type UploadResumeRecord, type UploadResumeStorage } from "../src/utils/upload-resume";
import { acquireUploadLease, releaseUploadLease, renewUploadLease } from "../src/utils/upload-coordinator";
import { sha256Hex } from "../src/utils/sha256";

class MemoryStorage implements UploadResumeStorage {
  private values = new Map<string, string>();
  get length() { return this.values.size; }
  key(index: number) { return Array.from(this.values.keys())[index] ?? null; }
  getItem(key: string) { return this.values.get(key) ?? null; }
  setItem(key: string, value: string) { this.values.set(key, value); }
  removeItem(key: string) { this.values.delete(key); }
}

function record(overrides: Partial<UploadResumeRecord> = {}): UploadResumeRecord {
  return { uploadId: "file-1", fileId: "file-1", roomCode: "1234", uploadUrl: "/upload", fileName: "demo.bin", fileSize: 20, lastModified: 10, expiresAt: 200, scope: "shared", recipientIds: [], chunkSize: 5, totalParts: 4, createdAt: 1, ...overrides };
}

describe("上传恢复元数据", () => {
  it("按房间和文件元数据匹配，并在过期时清理", () => {
    const storage = new MemoryStorage();
    saveUploadResume(record(), storage);
    saveUploadResume(record({ uploadId: "file-2", fileId: "file-2", expiresAt: 50 }), storage);
    expect(findUploadResume("1234", { name: "demo.bin", size: 20, lastModified: 10 }, 100_000, storage)?.uploadId).toBe("file-1");
    expect(listUploadResumes("1234", 100_000, storage).map((item) => item.uploadId)).toEqual(["file-1"]);
    expect(findUploadResume("1234", { name: "other.bin", size: 20, lastModified: 10 }, 100_000, storage)).toBeUndefined();
    removeUploadResume("1234", "file-1", storage);
    expect(listUploadResumes("1234", 100_000, storage)).toEqual([]);
  });
});

describe("上传标签页租约", () => {
  it("同一租约可续期和释放", () => {
    const storage = new MemoryStorage();
    expect(acquireUploadLease("file-1", 100, storage)).toBe(true);
    expect(renewUploadLease("file-1", 200, storage)).toBe(true);
    releaseUploadLease("file-1", storage);
    expect(acquireUploadLease("file-1", 300, storage)).toBe(true);
  });
});

describe("上传分片校验", () => {
  it("在非安全上下文也能计算 SHA-256", () => {
    expect(sha256Hex(new TextEncoder().encode("hello"))).toBe("2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824");
  });
});
