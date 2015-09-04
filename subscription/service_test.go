package subscription

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/oursky/ourd/oddb"
	"github.com/oursky/ourd/oddb/mock_oddb"
	. "github.com/smartystreets/goconvey/convey"
)

type notifyFunc func(device oddb.Device, notice Notice) error

func (f notifyFunc) Notify(device oddb.Device, notice Notice) error {
	return f(device, notice)
}

func TestService(t *testing.T) {
	Convey("Subscription Service", t, func() {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		timeNow = func() time.Time { return time.Unix(0x43b940e5, 0) }
		defer func() {
			timeNow = time.Now
		}()

		conn := mock_oddb.NewMockConn(ctrl)
		db := mock_oddb.NewMockDatabase(ctrl)

		service := &Service{
			ConnOpener: func() (oddb.Conn, error) { return conn, nil },
		}

		chch := make(chan chan oddb.RecordEvent, 1)
		conn.EXPECT().Subscribe(gomock.Any()).Do(func(recordEventCh chan oddb.RecordEvent) {
			chch <- recordEventCh
		})
		go service.Init().Listen()
		ch := <-chch

		record := oddb.Record{
			ID: oddb.NewRecordID("record", "0"),
		}
		subscription := oddb.Subscription{
			ID:       "subscriptionid",
			DeviceID: "deviceid",
		}
		device := oddb.Device{
			ID: "deviceid",
		}

		conn.EXPECT().PublicDB().Return(db)
		db.EXPECT().GetMatchingSubscriptions(&record).Return([]oddb.Subscription{
			subscription,
		})
		db.EXPECT().Conn().Return(conn)
		conn.EXPECT().GetDevice("deviceid", gomock.Any()).
			SetArg(1, device).
			Return(nil)

		Convey("sends notice", func() {
			var (
				d oddb.Device
				n Notice
			)
			done := make(chan bool)
			service.Notifier = notifyFunc(func(device oddb.Device, notice Notice) error {
				d = device
				n = notice
				done <- true
				return nil
			})

			ch <- oddb.RecordEvent{
				Record: &record,
				Event:  oddb.RecordCreated,
			}

			select {
			case <-done:
			case <-time.After(100 * time.Millisecond):
				t.Fatal("Receive no notices after 100 ms")
			}

			So(d, ShouldResemble, device)
			So(n, ShouldResemble, Notice{
				SeqNum:         0x43b940e50000,
				SubscriptionID: "subscriptionid",
				Event:          oddb.RecordCreated,
				Record:         &record,
			})
		})
	})
}
