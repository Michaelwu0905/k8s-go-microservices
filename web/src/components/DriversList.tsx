import {useEffect, useState} from 'react'
import {Button} from "@/components/ui/button"
import {ScrollArea} from "@/components/ui/scroll-area"
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card"
import {Avatar, AvatarFallback, AvatarImage} from "@/components/ui/avatar"
import {Car} from 'lucide-react'
import {Driver} from '@/hooks/useNearbyDrivers'
import {disableMapPropagation} from '@/lib/mapInteractions'

interface DriverListProps {
  drivers: Driver[]
  onSelectDriver: (driver: Driver) => void
}

export function DriverList({ drivers, onSelectDriver }: DriverListProps) {
  const [sortedDrivers, setSortedDrivers] = useState<Driver[]>([])

  useEffect(() => {
    const sorted = [...drivers].sort((a, b) => a.driver_id.localeCompare(b.driver_id))
    setSortedDrivers(sorted)
  }, [drivers])

  return (
    <Card ref={disableMapPropagation} className="w-full max-w-sm z-[9999] absolute top-0 right-0">
      <CardHeader>
        <CardTitle>Available Drivers</CardTitle>
        <CardDescription>Select a driver to request a ride</CardDescription>
      </CardHeader>
      <CardContent>
        <ScrollArea className="h-[300px]">
          {sortedDrivers.map((driver) => (
            <div key={driver.driver_id} className="flex items-center space-x-4 mb-4">
              <Avatar>
                <AvatarImage src={`https://api.dicebear.com/6.x/initials/svg?seed=${driver.driver_id}`} alt={driver.driver_id} />
                <AvatarFallback><Car /></AvatarFallback>
              </Avatar>
              <div className="flex-1 space-y-1">
                <p className="text-sm font-medium leading-none">{driver.driver_id}</p>
                <p className="text-sm text-muted-foreground">Toyota Camry</p>
                <div className="flex items-center">
                  {[...Array(5)].map((_, i) => (
                    <svg
                      key={i}
                      className={`w-4 h-4 ${i < 4 ? 'text-yellow-300' : 'text-gray-300'}`}
                      aria-hidden="true"
                      xmlns="http://www.w3.org/2000/svg"
                      fill="currentColor"
                      viewBox="0 0 22 20"
                    >
                      <path d="M20.924 7.625a1.523 1.523 0 0 0-1.238-1.044l-5.051-.734-2.259-4.577a1.534 1.534 0 0 0-2.752 0L7.365 5.847l-5.051.734A1.535 1.535 0 0 0 1.463 9.2l3.656 3.563-.863 5.031a1.532 1.532 0 0 0 2.226 1.616L11 17.033l4.518 2.375a1.534 1.534 0 0 0 2.226-1.617l-.863-5.03L20.537 9.2a1.523 1.523 0 0 0 .387-1.575Z" />
                    </svg>
                  ))}
                  <p className="ml-2 text-sm font-medium text-gray-500 dark:text-gray-400">4.5</p>
                </div>
              </div>
              <Button onClick={() => onSelectDriver(driver)}>Select</Button>
            </div>
          ))}
        </ScrollArea>
      </CardContent>
    </Card>
  )
}
