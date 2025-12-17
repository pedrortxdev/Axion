"use client";

import { useState, useEffect } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import axios from "axios";

interface IsoImage {
  name: string;
  size: number;
}

export default function ImagesPage() {
  const [images, setImages] = useState<IsoImage[]>([]);
  const [uploading, setUploading] = useState(false);
  const [uploadProgress, setUploadProgress] = useState(0);
  const [uploadSpeed, setUploadSpeed] = useState<string | null>(null);

  useEffect(() => {
    fetchImages();
  }, []);

  const fetchImages = async () => {
    try {
      const response = await axios.get("/api/isos");
      setImages(response.data);
    } catch (error) {
      console.error("Error fetching ISO images:", error);
    }
  };

  const handleFileUpload = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) return;

    const formData = new FormData();
    formData.append("iso", file);

    setUploading(true);
    setUploadProgress(0);
    setUploadSpeed(null);
    const startTime = Date.now();

    try {
      await axios.post("/api/isos/upload", formData, {
        onUploadProgress: (progressEvent) => {
          if (progressEvent.total) {
            const progress = Math.round((progressEvent.loaded * 100) / progressEvent.total);
            setUploadProgress(progress);

            const elapsedTime = (Date.now() - startTime) / 1000; // in seconds
            const speed = progressEvent.loaded / elapsedTime; // bytes per second
            setUploadSpeed(formatSpeed(speed));
          }
        },
      });
      fetchImages(); // Refresh the list after upload
    } catch (error) {
      console.error("Error uploading ISO image:", error);
    } finally {
      setUploading(false);
    }
  };

  const formatSize = (bytes: number) => {
    if (bytes === 0) return "0 Bytes";
    const k = 1024;
    const sizes = ["Bytes", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
  };

  const formatSpeed = (bytesPerSecond: number) => {
    if (bytesPerSecond < 1024) {
        return `${bytesPerSecond.toFixed(2)} B/s`;
    } else if (bytesPerSecond < 1024 * 1024) {
        return `${(bytesPerSecond / 1024).toFixed(2)} KB/s`;
    } else {
        return `${(bytesPerSecond / (1024 * 1024)).toFixed(2)} MB/s`;
    }
  };


  return (
    <div className="container mx-auto p-4">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle>ISO Images</CardTitle>
          <input
            type="file"
            id="file-upload"
            className="hidden"
            onChange={handleFileUpload}
            accept=".iso"
          />
          <label htmlFor="file-upload">
            <Button asChild>
              <span>Upload ISO</span>
            </Button>
          </label>
        </CardHeader>
        <CardContent>
          {uploading && (
            <div className="mb-4">
              <p>Uploading...</p>
              <div className="w-full bg-gray-200 rounded-full h-2.5 dark:bg-gray-700">
                <div
                  className="bg-blue-600 h-2.5 rounded-full"
                  style={{ width: `${uploadProgress}%` }}
                ></div>
              </div>
              <p>{uploadProgress}% {uploadSpeed && `(${uploadSpeed})`}</p>
            </div>
          )}
          <ul>
            {images.map((image) => (
              <li key={image.name} className="flex justify-between items-center p-2 border-b">
                <span>{image.name}</span>
                <span>{formatSize(image.size)}</span>
              </li>
            ))}
          </ul>
        </CardContent>
      </Card>
    </div>
  );
}
