
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Copy } from "lucide-react";

interface InstanceInfoCardProps {
  node: string;
  ipAddress: string;
  vcpu: number;
  ram: string;
  disk: string;
}

export function InstanceInfoCard({ node, ipAddress, vcpu, ram, disk }: InstanceInfoCardProps) {
  const handleCopy = () => {
    navigator.clipboard.writeText(ipAddress);
  };

  return (
    <Card className="bg-zinc-900/50 border-zinc-800">
      <CardHeader>
        <CardTitle className="text-lg font-semibold">Instance Info</CardTitle>
      </CardHeader>
      <CardContent className="grid gap-4 text-sm">
        <div className="flex justify-between">
          <span className="text-zinc-400">Node:</span>
          <span>{node}</span>
        </div>
        <div className="flex justify-between items-center">
          <span className="text-zinc-400">IP Address:</span>
          <div className="flex items-center gap-2">
            <span>{ipAddress}</span>
            <Button variant="ghost" size="icon" onClick={handleCopy} className="h-7 w-7">
              <Copy className="h-4 w-4" />
            </Button>
          </div>
        </div>
        <div className="flex justify-between">
          <span className="text-zinc-400">vCPU:</span>
          <span>{vcpu} vCores</span>
        </div>
        <div className="flex justify-between">
          <span className="text-zinc-400">RAM:</span>
          <span>{ram}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-zinc-400">Disk:</span>
          <span>{disk}</span>
        </div>
      </CardContent>
    </Card>
  );
}
